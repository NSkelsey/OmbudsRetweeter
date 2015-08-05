package main

import (
	"bufio"
	"container/list"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"math/rand"
	"time"

	"github.com/btcsuite/btcd/btcjson/v2/btcjson"
	"github.com/mrjones/oauth"
)

// Twitter's api rate limiting window
var windowDur time.Duration = 15 * time.Minute

// A json representation of Twitter's 'statuses' as they are pushed out as JSON via
// their endpoints
type Tweet struct {
	Text      string     `json:"text"`      // The content of the tweet
	Id        int        `json:"id"`        // Unique Id for the given tweet.
	User      UserFields `json:"user"`      // The user object for this tweet.
	Ents      Entities   `json:"entities"`  // Contains objects within the tweet
	Retweeted bool       `json:"retweeted"` // Flag to indicate if status is a retweet.
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

var retweetText []string = []string{
	"Wow %s, thanks!",
	"%s said it like he meant it.",
	"Yep... %s...",
	"Very true %s, very true",
	"As I robot I can relate",
	"This has nothing to do with anything",
	"Great post!",
	"Great tweet!",
	"Good tweet!",
	"Wonderful",
	"You are so handy!",
	"Great!",
	"Word.",
	"The Truth",
	"%s is the cat's pajamas",
	"%s is the cat's meow",
	"%s is the bee's knees",
	"This tweet is right bully",
	"Rootin' tootin' %s is right!",
	"Say $s what gives?",
	"Heavens to betsy! %s is right!",
	"Constructive...!",
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
	cfg      *config
	token    *oauth.AccessToken
	consumer *oauth.Consumer
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

	c := oauth.NewConsumer(
		cfg.ConsumerKey,
		cfg.ConsumerSecret,
		oauth.ServiceProvider{
			RequestTokenUrl:   "https://api.twitter.com/oauth/request_token",
			AuthorizeTokenUrl: "https://api.twitter.com/oauth/authorize",
			AccessTokenUrl:    "https://api.twitter.com/oauth/access_token",
		})

	s := &server{
		cfg:        cfg,
		token:      tok,
		consumer:   c,
		tweetCache: list.New(),
	}

	return s, nil
}

func (s *server) Start() {
	s.listenTwitterStream()
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
			log.Fatal(err)
		}
		if s.detectTweet(str) {
			err := s.handleTweet(str)
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

// Informs the user that their tweet was not backed up.
func (s *server) storeFailed(tweet *Tweet) error {
	status := "Woops. Looks like I can't back that up... https://twitter.com/%s/status/%d"
	status = fmt.Sprintf(status, tweet.User.ScreenName, tweet.Id)
	resp, err := s.consumer.Post(
		"https://api.twitter.com/1.1/statuses/update.json",
		map[string]string{
			"status": status,
		},
		s.token,
	)
	log.Printf("%v\n", resp)

	if err != nil {
		return err
	}
	return nil
}

// Formulates a retweet and posts it to Twitter. This mimics what is posted in
// the block chain.
func (s *server) retweet(txid, rtext string, tweet *Tweet) error {

	s.cacheSentTweet(tweet)

	status := fmt.Sprintf("%s it's stored on the web @ http://relay.getombuds.org and in the public record. https://twitter.com/%s/status/%d",
		tweet.User.ScreenName, tweet.User.ScreenName, tweet.Id)
	_, err := s.consumer.Post(
		"https://api.twitter.com/1.1/statuses/update.json",
		map[string]string{
			"status": status,
		},
		s.token,
	)

	if err != nil {
		return err
	}
	return nil
}

// handleTweet takes the raw json of a tweet and produces the output in the
// blockchain and on twitter. Cases of failing bulletins, failing tweets and
// unexpected scenarios are handled.
func (s *server) handleTweet(str string) error {
	fmt.Printf("Here's the raw tweet: %s\n", str)
	tweet := &Tweet{}
	err := json.Unmarshal([]byte(str), tweet)
	if err != nil {
		return err
	}
	fmt.Printf("Here's the parsed tweet: %s\n", tweet)

	if s.canSend() {

		unlockCmd := s.makeUnlockCmd()
		_, err = s.rpcsend(unlockCmd)
		if err != nil {
			log.Printf("Error unlocking the wallet: %s\n", err)
			s.storeFailed(tweet)
			return nil
		}

		bltnCmd, rtText := s.makeBltn(tweet)
		txid, err := s.rpcsend(bltnCmd)
		if err != nil {
			log.Printf("Sending the tweet failed: %s\n", err)
			s.storeFailed(tweet)
			return nil
		}

		lockCmd := btcjson.NewWalletLockCmd()
		_, err = s.rpcsend(lockCmd)
		if err != nil {
			log.Printf("Error locking the wallet: %s\n", err)
			s.storeFailed(tweet)
			return nil
		}

		err = s.retweet(txid, rtText, tweet)
		if err != nil {
			log.Printf("Retweet failed: %s\n", err)
			return nil
		}

	} else {
		log.Println("Ignoring tweet by: @%s", tweet.User.ScreenName)
	}
	return nil
}

func formatStatusText(tweet *Tweet) string {
	orig := tweet.Text
	s := ""

	pos := 0
	for _, HashTag := range tweet.Ents.HashTags {
		lnk := fmt.Sprintf("[#%s](https://twitter.com/hashtag/%s?src=hash&f=tweets)",
			HashTag.Text, HashTag.Text)
		s += orig[pos:HashTag.Indices[0]] + lnk
		pos = HashTag.Indices[1]
	}
	s += orig[pos:]
	return s
}

func (s *server) makeBltn(tweet *Tweet) (interface{}, string) {

	rtText := fmt.Sprintf("First seen on [Twitter](https://twitter.com/%s/status/%d)",
		tweet.User.ScreenName, tweet.Id)

	richText := formatStatusText(tweet)

	msg := fmt.Sprintf("%s \n\n\n<code>\n%s\n</code>", rtText, richText)
	board := fmt.Sprintf("Testnet %s Log", s.cfg.Hashtag)

	bltn, _ := btcjson.NewCmd("sendbulletin", s.cfg.SendAddress, board, msg)

	return bltn, rtText
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
