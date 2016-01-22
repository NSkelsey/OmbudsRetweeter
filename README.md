# OmbudsRetweeter
This is a server daemon for replicating content from Twitter into the public record.

### How does it work?

When a tweet with `#RecordThisPlease` and the server's Twitter bot username is created, this bot does three things.
1) It turns the tweet into a bulletin, 2) it ensures that bulletin is stored in the public record and, 3) it replies to the user informing them if it worked or not. 

This bulletin can be viewed in the public record by clients that host it or by a web service that relays it. This repository also contains code for a web service that relays it. An example of that service live on the internet is sporadically viewable at [relay.getombuds.org](http://relay.getombuds.org). 

If none of that made much sense, the picture below may help explain the architecture of the systems involved. Steps 1, 2, and, 3 are labeled below.

![System Architecture](http://i.imgur.com/tU10k7C.jpg)

### How Do I Use It?
It's very simple. Tweet at an OmbudsRetweeter bot with `#RecordThisPlease`

The image below is an example of an exchange between a twitter user and our testnetwork retweeting bot.

Note that the tweet by `@_nskelsey_` is missing `#RecordThisPlease`.

```

```

![Stored tweet](http://i.imgur.com/XFjzkRy.png | width=200)

Alternatively, you can record someone else's tweet by replying to their original tweet and mentioning the retweet bot (and including `#RecordThisPlease`). Below is an example exchange where @_nskelsey_ permanently records what @askuck ~~twat~~ tweeted in the public record.

```

```

![Stored tweet](http://i.imgur.com/9Y6pyBE.png | width=200)

### Running a Bot

If you'd like to run this software yourself you are going to have to install some large dependecies. 

An incomplete list follows below:
- Golang
- Nginx
- An Ombuds Full Node
- A BTC-RPC v1 Compliant Bitcoin Wallet

If you are serious about it just reach out to @NSkelsey or @alexkuck. We are happy to help!
If you would like to know more about ombuds visit us on the [web](https://getombuds.org).
