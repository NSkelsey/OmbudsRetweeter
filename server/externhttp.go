package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"math/rand"
	"strconv"
)

// GetTweet queries twitter's api for the tweet specified by id
func (s *server) getTweet(id string) (*Tweet, error) {

	url := fmt.Sprintf("https://api.twitter.com/1.1/statuses/show/%s.json", id)
	response, err := s.consumer.Get(url, map[string]string{}, s.token)
	if err != nil {
		return nil, err
	}

	if response.StatusCode != 200 {
		return nil, fmt.Errorf("Could not get tweet: %s", id)
	}
	b, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}

	tweet := &Tweet{}
	if err = json.Unmarshal(b, tweet); err != nil {
		return nil, err
	}

	return tweet, nil
}

var retweetFailed []string = []string{
	"Ouch... something broke. Please let someone know!",
	"Nope, that didn't work. Sorry!",
	"Recording the tweet failed. Apologies.",
	"That did not work. Sorry!",
	"Maybe try again? It seems broken to me.",
	"Definitely not working...sorry!",
	"Error! A human needs to fix this.",
}

// Informs the user that their tweet was not backed up.
func (s *server) storeFailed(tweet *Tweet) error {
	t := len(retweetFailed)
	status := fmt.Sprintf("@%s %s", tweet.User.ScreenName, retweetFailed[rand.Intn(t)])
	resp, err := s.consumer.Post(
		"https://api.twitter.com/1.1/statuses/update.json",
		map[string]string{
			"status":                status,
			"in_reply_to_status_id": strconv.Itoa(tweet.Id),
		},
		s.token,
	)
	if err != nil {
		log.Printf("FAILED:\nReTweet:%s\nResp:%s\n", tweet, resp.Status)
		return err
	}
	log.Println("Success: Retweeted the error")
	return nil
}

// Formulates a response to a single tweet and posts it to Twitter. This links to what is stored in
// the block chain.
func (s *server) respondWithStatus(tweet *Tweet, storedParent bool) error {

	s.cacheSentTweet(tweet)

	status := fmt.Sprintf("@%s Your tweet has been sent to the public record. You can see its status here: %s",
		tweet.User.ScreenName, s.cfg.RelayUrl)
	if storedParent {
		status = fmt.Sprintf("@%s the tweet you originally replied to has been sent to the public record. See its status here: %s", tweet.User.ScreenName, s.cfg.RelayUrl)
	}

	_, err := s.consumer.Post(
		"https://api.twitter.com/1.1/statuses/update.json",
		map[string]string{
			"status":                status,
			"in_reply_to_status_id": strconv.Itoa(tweet.Id),
		},
		s.token,
	)

	if err != nil {
		return err
	}
	return nil
}
