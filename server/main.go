package main

import (
	"bufio"
	"container/list"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"math"
	"math/rand"
	"time"

	"github.com/btcsuite/btcd/btcjson"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcrpcclient"
	"github.com/mrjones/oauth"
	"github.com/soapboxsys/ombudslib/ombpublish"
	"github.com/soapboxsys/ombudslib/ombwire"
)

// Twitter's api rate limiting window
var windowDur time.Duration = 15 * time.Minute

// The active BitcoinNet the application is interfacing with
var activeNet chaincfg.Params

// A json representation of Twitter's 'statuses' as they are pushed out as JSON via
// their endpoints
type Tweet struct {
	Text        string     `json:"text"`                                // The content of the tweet
	Id          int        `json:"id"`                                  // Unique Id for the given tweet.
	User        UserFields `json:"user"`                                // The user object for this tweet.
	Ents        Entities   `json:"entities"`                            // Contains objects within the tweet
	Retweeted   bool       `json:"retweeted"`                           // Flag to indicate if status is a retweet.
	ParentId    int        `json:"in_reply_to_status_id",omitempty`     // A field that indicates if the tweet is a reply
	ParentIdStr string     `json:"in_reply_to_status_id_str",omitempty` // A field that indicates if the tweet is a reply
}

type UserFields struct {
	ScreenName string `json:"screen_name"` // Users Twitter handle
}

type Entities struct {
	HashTags []HashTag `json:"hashtags"` // The list of hashtags within the tweet
}

type HashTag struct {
	Text    string `json:"text"`    // The text minus the # of the hashtag
	Indices []int  `json:"indices"` // Opening and closing position of the hashtag.
}

func (s *server) detectTweet(str string) bool {
	tweet := &Tweet{}
	err := json.Unmarshal([]byte(str), tweet)
	if err != nil {
		return false
	}

	// Ignore tweets from the bot itself.
	if tweet.User.ScreenName == s.cfg.BotScreenName {
		return false
	}

	// Ignore retweets as well.
	if tweet.Retweeted {
		return false
	}

	return true
}

type server struct {
	cfg       *config
	rpcClient *btcrpcclient.Client
	pubParams *ombpublish.Params
	token     *oauth.AccessToken
	consumer  *oauth.Consumer
	// The number of tweets we tried to store
	cnt        int
	tweetCache *list.List // All tweets sent in the last 15 minutes.
}

func newServer(cfg *config) (*server, error) {

	b, err := ioutil.ReadFile(cfg.AccessTokenFile)
	if err != nil {
		return nil, err
	}

	tok := &oauth.AccessToken{}

	err = json.Unmarshal(b, tok)
	if err != nil {
		return nil, err
	}

	// Setup Twitter Oauth
	c := oauth.NewConsumer(
		cfg.ConsumerKey,
		cfg.ConsumerSecret,
		oauth.ServiceProvider{
			RequestTokenUrl:   "https://api.twitter.com/oauth/request_token",
			AuthorizeTokenUrl: "https://api.twitter.com/oauth/authorize",
			AccessTokenUrl:    "https://api.twitter.com/oauth/access_token",
		},
	)

	client := createRPCClient(cfg)
	pubParams := ombpublish.NormalParams(&activeNet, cfg.WalletPassphrase)
	pubParams.Verbose = false

	s := &server{
		cfg:        cfg,
		rpcClient:  client,
		pubParams:  &pubParams,
		token:      tok,
		consumer:   c,
		tweetCache: list.New(),
	}

	return s, nil
}

func createRPCClient(cfg *config) *btcrpcclient.Client {

	certs, err := ioutil.ReadFile(cfg.RPCCert)
	if err != nil {
		log.Fatal(err)
	}

	connCfg := &btcrpcclient.ConnConfig{
		Host:         cfg.RPCServer,
		User:         cfg.RPCUser,
		Pass:         cfg.RPCPassword,
		HTTPPostMode: true,
		Certificates: certs,
	}

	client, err := btcrpcclient.New(connCfg, nil)
	if err != nil {
		log.Fatal(err)
	}

	_, err = client.GetBlockCount()
	if err != nil {
		log.Fatal(err)
	}

	return client
}

func (s *server) Start() {
	s.listenTwitterStream()
}

// tryToReconnect uses an exponential backoff to try and reconnect to Twitter's
// status stream. It will try for around 3 days to connect before giving up
// completely.
func (s *server) tryToReconnect() (*bufio.Reader, error) {
	for i := 0; i < 30; i++ {
		response, err := s.consumer.Get(
			//"https://stream.twitter.com/1.1/statuses/filter.json",
			"https://userstream.twitter.com/1.1/user.json",
			map[string]string{"track": s.cfg.Hashtag},
			s.token)

		if err != nil {
			// Backoffs at 1 + 1*(1+.10)^i
			t := int(1000 + 1000*math.Pow(1+0.10, float64(i)))
			backoff := time.Duration(t) * time.Millisecond
			log.Printf("Saw: %s. Backing off for: %s\n", err, backoff.String())
			time.Sleep(backoff)
			continue
		}

		reader := bufio.NewReader(response.Body)
		log.Printf("Success: Reconnected to %s stream\n", s.cfg.Hashtag)
		return reader, nil
	}
	return nil, fmt.Errorf("Could not reconnect after retrying...")
}

func (s *server) listenTwitterStream() {
	response, err := s.consumer.Get(
		"https://stream.twitter.com/1.1/statuses/filter.json",
		map[string]string{"track": s.cfg.Hashtag},
		s.token)

	if err != nil {
		log.Fatal(err)
	}
	defer response.Body.Close()

	reader := bufio.NewReader(response.Body)
	log.Printf("Connected to %s stream\n", s.cfg.Hashtag)

	for {
		str, err := reader.ReadString('\n')
		if err != nil {
			log.Printf("Reading from twitter threw %s\n", err)
			reader, err = s.tryToReconnect()
			if err != nil {
				log.Fatal(err)
			}
			continue
		}
		if s.detectTweet(str) {
			err := s.handleIncomingTweet(str)
			if err != nil {
				log.Println(err.Error())
			}
		}
	}
}

// holds meta data for the tweet cache.
type cacheRecord struct {
	ts    time.Time
	tweet *Tweet
}

func (s *server) cacheSentTweet(t *Tweet) {
	if s.tweetCache.Len() >= 24 {
		// Trim off the tweets that are older than the window.
		cur := s.tweetCache.Back()
		for cur.Next() != nil {
			next := cur.Next()
			curTweet := (cur.Value).(*cacheRecord)
			if curTweet.ts.Add(windowDur).Before(time.Now()) {
				// Current element is too old. Expire it.
				s.tweetCache.Remove(cur)
			}
			cur = next
		}
	}
	r := cacheRecord{
		ts:    time.Now(),
		tweet: t,
	}

	s.tweetCache.PushFront(&list.Element{Value: r})
}

// canSend ensures that no rate limits have been exceeded.
func (s *server) canSend() bool {
	if s.tweetCache.Len() < 24 {
		// Cache is not full.
		return true
	}

	// This is the last element in the list.
	lastCacheR := s.tweetCache.Back().Value.(*cacheRecord)

	window := lastCacheR.ts.Add(windowDur)
	if time.Now().Before(window) {
		// We have exceed the window's rate of messages. Do not tweet.
		return false
	}

	return true
}

// handleIncomingTweet takes the raw json of a tweet and produces the output in the
// blockchain and on twitter. Cases of failing bulletins, failing tweets and
// unexpected scenarios are handled.
func (s *server) handleIncomingTweet(str string) error {
	//fmt.Printf("Here's the raw tweet: %s\n", str)
	tweet := &Tweet{}
	err := json.Unmarshal([]byte(str), tweet)
	if err != nil {
		return err
	}
	log.Printf("Info: pushed tweet by @%s id:[%d]\n", tweet.User.ScreenName, tweet.Id)

	if s.canSend() {
		// Figure out if the tweet is a reply and if so, record what the
		// original poster said. Use the in_reply_to_status field to determine
		// if the server will respond to the original tweet or its parent.
		// All responses via tweet are too the person that tweeted at the bot
		// though.

		// The tweet that we are going to backup
		targetTweet := tweet
		if tweet.ParentIdStr != "" {
			targetTweet, err = s.getTweet(tweet.ParentIdStr)
			if err != nil {
				log.Printf("Failed: could not get parent tweet: %s", err)
				return nil
			}
		}
		storedParent := targetTweet.Id != tweet.Id

		wireBltn := s.makeBltn(targetTweet)
		txid, err := ombpublish.PublishBulletin(s.rpcClient, wireBltn, *s.pubParams)
		if err != nil || txid == nil {
			log.Printf("Failed: sending the bltn: %s\n", err)
			s.storeFailed(tweet)
			return nil
		}
		log.Printf("Success: Stored bltn: %s", txid)

		err = s.respondWithStatus(tweet, storedParent)
		if err != nil {
			log.Printf("Failed: Retweet failed: %s\n", err)
			return nil
		}
		log.Println("Success: Responded via Twitter")

	} else {
		log.Println("Failed: Ignoring tweet by: @%s", tweet.User.ScreenName)
	}
	return nil
}

func (s *server) makeBltn(tweet *Tweet) *ombwire.Bulletin {
	sn := tweet.User.ScreenName

	// Schema for bltns generated by this tool is:
	// #RTMirror of [@username](https://twtr.com/uname/status/345345345343433)
	// TWEET BODY TWEET BODY
	// TWEET BODY TWEET BODY
	postLink := fmt.Sprintf("[@%s](https://twitter.com/%s/status/%d)", sn, tweet.Id)

	msg := fmt.Sprintf("#RTMirror of %s\n%s", postLink, tweet.Text)

	now := uint64(time.Now().Unix())
	bltn := ombwire.NewBulletin(msg, now, nil)

	return bltn
}

func (s *server) makeUnlockCmd() interface{} {
	cmd, _ := btcjson.NewCmd("walletpassphrase", s.cfg.WalletPassphrase, 5)
	return cmd
}

func main() {
	rand.Seed(time.Now().Unix())
	cfg, _, err := loadConfig()
	if err != nil {
		log.Fatal(err)
	}

	s, err := newServer(cfg)
	if err != nil {
		log.Fatal(err)
	}

	s.Start()

}
