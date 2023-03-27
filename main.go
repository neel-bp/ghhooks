package main

import (
	"flag"
	"log"
	"net/http"
	"os"

	"ghhooks.com/hook/core"
	"ghhooks.com/hook/httpinterface"
	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
)

// TODO: flag to control if github commit badge should be updated or not
// TODO: gracefull shutdown that waits for server to shutdown and also waits for current build to finish
func main() {
	configFileLocation := flag.String("config", "example.toml", "location of config file")
	httpLogger := flag.Bool("httplog", true, "log http requests (webhook push event and status request)")
	flag.Parse()

	l := log.New(os.Stdout, "", 0)
	err := core.ServerInit(*configFileLocation, l)
	if err != nil {
		log.Fatal(err)
	}
	r := mux.NewRouter()
	httpinterface.RouterInit(r)
	var handler http.Handler = r
	if *httpLogger {
		handler = handlers.LoggingHandler(os.Stdout, handler)
	}

	srv := &http.Server{
		Handler: handler,srv.S
		Addr:    ":4444",
	}
	log.Printf("listening on %s", srv.Addr)
	log.Fatal(srv.ListenAndServe())
}
