// Copyright (c) 2013-2015 Conformal Systems LLC.
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"

	"github.com/btcsuite/btcd/btcjson"
	"github.com/btcsuite/btcd/chaincfg"
	flags "github.com/btcsuite/go-flags"
	"github.com/soapboxsys/ombudslib/ombutil"
)

const (
	// unusableFlags are the command usage flags which this utility are not
	// able to use.  In particular it doesn't support websockets and
	// consequently notifications.
	unusableFlags = btcjson.UFWebsocketOnly | btcjson.UFNotification
)

var (
	ombudsNodeHome        = ombutil.AppDataDir("ombnode", false)
	btcdHomeDir           = filepath.Join(ombudsNodeHome, "node")
	retweeterHomeDir      = ombutil.AppDataDir("retweeters", false)
	defaultConfigFile     = filepath.Join(retweeterHomeDir, "serv.conf")
	defaultRPCServer      = "localhost"
	defaultRPCCertFile    = filepath.Join(ombudsNodeHome, "rpc.cert")
	defaultWalletCertFile = filepath.Join(ombudsNodeHome, "rpc.cert")
	defaultAccessToken    = filepath.Join(retweeterHomeDir, "token.json")
)

// config defines the configuration options for retweeter.
//
// See loadConfig for details on the configuration load process.
type config struct {
	ConfigFile  string `short:"C" long:"configfile" description:"Path to configuration file"`
	RPCUser     string `short:"u" long:"rpcuser" description:"RPC username"`
	RPCPassword string `short:"P" long:"rpcpass" default-mask:"-" description:"RPC password"`
	RPCServer   string `short:"s" long:"rpcserver" description:"RPC server to connect to"`
	RPCCert     string `short:"c" long:"rpccert" description:"RPC server certificate chain for validation"`
	NoTLS       bool   `long:"notls" description:"Disable TLS"`
	TestNet3    bool   `long:"testnet" description:"Connect to testnet"`

	BotScreenName    string `long:"botscreenname" description:"The Twitter handle of the bot."`
	ConsumerKey      string `long:"consumerkey" description:"Twitter API consumer key"`
	ConsumerSecret   string `long:"consumersecret" description:"Twitter API consumer secret"`
	AccessTokenFile  string `long:"accesstoken" short:"t" description:"The name of the file the access token is stored in."`
	Hashtag          string `long:"hashtag" short:"h" description:"The hashtag to track."`
	WalletPassphrase string `long:"walletpassphrase" description:"The wallet's passphrase for sending."`
}

func hasField(name, s string) {
	if s == "" {
		fmt.Printf("Include omitted config: %s\n", name)
		os.Exit(1)
	}
}

// normalizeAddress returns addr with the passed default port appended if
// there is not already a port specified.
func normalizeAddress(addr string, useTestNet3, useSimNet, useWallet bool) string {
	_, _, err := net.SplitHostPort(addr)
	if err != nil {
		var defaultPort string
		switch {
		case useTestNet3:
			if useWallet {
				defaultPort = "18332"
			} else {
				defaultPort = "18334"
			}
		case useSimNet:
			if useWallet {
				defaultPort = "18554"
			} else {
				defaultPort = "18556"
			}
		default:
			if useWallet {
				defaultPort = "8332"
			} else {
				defaultPort = "8334"
			}
		}

		return net.JoinHostPort(addr, defaultPort)
	}
	return addr
}

// cleanAndExpandPath expands environement variables and leading ~ in the
// passed path, cleans the result, and returns it.
func cleanAndExpandPath(path string) string {
	// Expand initial ~ to OS specific home directory.
	if strings.HasPrefix(path, "~") {
		homeDir := filepath.Dir(retweeterHomeDir)
		path = strings.Replace(path, "~", homeDir, 1)
	}

	// NOTE: The os.ExpandEnv doesn't work with Windows-style %VARIABLE%,
	// but they variables can still be expanded via POSIX-style $VARIABLE.
	return filepath.Clean(os.ExpandEnv(path))
}

// loadConfig initializes and parses the config using a config file and command
// line options.
//
// The configuration proceeds as follows:
//	1) Start with a default config with sane settings
//	2) Pre-parse the command line to check for an alternative config file
//	3) Load configuration file overwriting defaults with any specified options
//	4) Parse CLI options and overwrite/add any specified options
//
// The above results in functioning properly without any config settings
// while still allowing the user to override settings with config files and
// command line options.  Command line options always take precedence.
func loadConfig() (*config, []string, error) {
	// Default config.
	cfg := config{
		ConfigFile:      defaultConfigFile,
		RPCServer:       defaultRPCServer,
		RPCCert:         defaultRPCCertFile,
		AccessTokenFile: defaultAccessToken,
	}

	// Create the home directory if it doesn't already exist.
	err := os.MkdirAll(retweeterHomeDir, 0700)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(-1)
	}

	// Pre-parse the command line options to see if an alternative config
	// file, the version flag, or the list commands flag was specified.  Any
	// errors aside from the help message error can be ignored here since
	// they will be caught by the final parse below.
	preCfg := cfg
	preParser := flags.NewParser(&preCfg, flags.HelpFlag)
	_, err = preParser.Parse()
	if err != nil {
		if e, ok := err.(*flags.Error); ok && e.Type == flags.ErrHelp {
			return nil, nil, err
		}
	}

	// Show the version and exit if the version flag was specified.
	appName := filepath.Base(os.Args[0])
	appName = strings.TrimSuffix(appName, filepath.Ext(appName))
	usageMessage := fmt.Sprintf("Use %s -h to show options", appName)

	// Load additional config from file.
	parser := flags.NewParser(&cfg, flags.Default)
	err = flags.NewIniParser(parser).ParseFile(preCfg.ConfigFile)
	if err != nil {
		if _, ok := err.(*os.PathError); !ok {
			fmt.Fprintf(os.Stderr, "Error parsing config file: %v\n",
				err)
			fmt.Fprintln(os.Stderr, usageMessage)
			return nil, nil, err
		}
	}

	// Parse command line options again to ensure they take precedence.
	remainingArgs, err := parser.Parse()
	if err != nil {
		if e, ok := err.(*flags.Error); !ok || e.Type != flags.ErrHelp {
			fmt.Fprintln(os.Stderr, usageMessage)
		}
		return nil, nil, err
	}

	// Set activeNet for the application
	activeNet = chaincfg.MainNetParams
	if cfg.TestNet3 {
		activeNet = chaincfg.TestNet3Params
	}

	// Handle environment variable expansion in the RPC certificate path.
	cfg.RPCCert = cleanAndExpandPath(cfg.RPCCert)

	// Add default port to RPC server based on --testnet and --wallet flags
	// if needed.
	cfg.RPCServer = normalizeAddress(cfg.RPCServer, cfg.TestNet3,
		false, true)

	hasField("hashtag", cfg.Hashtag)
	//hasField("sending address", cfg.SendAddress)
	hasField("consumer key", cfg.ConsumerKey)
	hasField("consumer secret", cfg.ConsumerSecret)
	hasField("wallet passphrase", cfg.WalletPassphrase)

	return &cfg, remainingArgs, nil
}
