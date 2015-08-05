package main

import (
	"flag"
	"log"
	"net/http"
	"strconv"
    "path"

	"github.com/NSkelsey/ahimsarest"
	"github.com/soapboxsys/ombudslib/pubrecdb"
	"github.com/soapboxsys/ombudslib/ombutil"
)

func main() {
    defaultAppPath := ombutil.AppDataDir("Ombudscore", false)
	var DBPath *string = flag.String(
		"dbpath",
		path.Join(defaultAppPath, "node", "pubrecord.db"),
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
    mux.Handle("/", http.FileServer(http.Dir("./static")))

	host := "localhost:" + strconv.Itoa(*port)

	log.Println(http.ListenAndServe(host, mux))
}
