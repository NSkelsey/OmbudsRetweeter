package main

import (
	"flag"
	"log"
	"net/http"
	"path"
	"strconv"

	"github.com/soapboxsys/ombudslib/jsonapi"
	"github.com/soapboxsys/ombudslib/ombutil"
	"github.com/soapboxsys/ombudslib/pubrecdb"
)

func main() {
	defaultAppPath := ombutil.AppDataDir("ombnode", false)
	var DBPath *string = flag.String(
		"pubrecpath",
		path.Join(defaultAppPath, "data", "testnet", "pubrecord.db"),
		"Path to the public record database",
	)

	var port *int = flag.Int(
		"port",
		8080,
		"Port to listen on.",
	)

	flag.Parse()

	db, err := pubrecdb.LoadDB(*DBPath)
	if err != nil {
		log.Fatal(err)
	}

	apiPrefix := "/api/"
	api := jsonapi.Handler(apiPrefix, db)

	mux := http.NewServeMux()
	mux.Handle(apiPrefix, api)
	mux.Handle("/", http.FileServer(http.Dir("./static")))

	host := "0.0.0.0:" + strconv.Itoa(*port)
	log.Printf("Server listening on: %s\n", host)

	log.Println(http.ListenAndServe(host, mux))
}
