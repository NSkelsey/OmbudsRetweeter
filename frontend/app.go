package main

import (
	"flag"
	"log"
	"net/http"
	"strconv"

	"github.com/NSkelsey/ahimsarest"
	"github.com/soapboxsys/ombudslib/pubrecdb"
)

func main() {
	var DBPath *string = flag.String(
		"dbpath",
		"~/.ombudscore/node/pubrec.db",
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
	api := ahimsarest.Handler(apiPrefix, db)

	mux := http.NewServeMux()
	mux.Handle(apiPrefix, api)

	host := "localhost:" + strconv.Itoa(*port)

	log.Println(http.ListenAndServe(host, mux))
}
