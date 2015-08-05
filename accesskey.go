package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"

	"github.com/mrjones/oauth"
)

func Usage() {
	fmt.Println("Usage:")
	fmt.Print("go run examples/twitter/twitter.go")
	fmt.Print("  --consumerkey <consumerkey>")
	fmt.Println("  --consumersecret <consumersecret>")
	fmt.Println("  -t <accesstoken.json>")
	fmt.Println("")
	fmt.Println("In order to get your consumerkey and consumersecret, you must register an 'app' at twitter.com:")
	fmt.Println("https://dev.twitter.com/apps/new")
}

func main() {
	var consumerKey *string = flag.String(
		"consumerkey",
		"",
		"Consumer Key from Twitter. See: https://dev.twitter.com/apps/new")

	var consumerSecret *string = flag.String(
		"consumersecret",
		"",
		"Consumer Secret from Twitter. See: https://dev.twitter.com/apps/new")

	var outFileString *string = flag.String(
		"t",
		"outfile",
		"File to store the access token in.")

	flag.Parse()

	if len(*consumerKey) == 0 || len(*consumerSecret) == 0 {
		fmt.Println("You must set the --consumerkey and --consumersecret flags.")
		fmt.Println("---")
		Usage()
		os.Exit(1)
	}

	c := oauth.NewConsumer(
		*consumerKey,
		*consumerSecret,
		oauth.ServiceProvider{
			RequestTokenUrl:   "https://api.twitter.com/oauth/request_token",
			AuthorizeTokenUrl: "https://api.twitter.com/oauth/authorize",
			AccessTokenUrl:    "https://api.twitter.com/oauth/access_token",
		})

	requestToken, u, err := c.GetRequestTokenAndUrl("oob")
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("(1) Go to: " + u)
	fmt.Println("(2) Grant access, you should get back a verification code.")
	fmt.Println("(3) Enter that verification code here: ")

	verificationCode := ""
	fmt.Scanln(&verificationCode)

	accessToken, err := c.AuthorizeToken(requestToken, verificationCode)
	if err != nil {
		log.Fatal(err)
	}

	tokJson, err := json.Marshal(accessToken)
	if err != nil {
		log.Fatal(err)
	}

	if *outFileString != "" {
		err := ioutil.WriteFile(*outFileString, tokJson, 0600)
		if err != nil {
			log.Fatal(err)
		}
	} else {
		fmt.Printf("\n%s\n", tokJson)
	}
}
