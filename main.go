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

//TODO: flag to control if github commit badge should be updated or not
func main() {
	configFileLocation := flag.String("config", "example.toml", "location of config file")
	flag.Parse()

	l := log.New(os.Stdout, "", 0)
	err := core.ServerInit(*configFileLocation, l)
	if err != nil {
		log.Fatal(err)
	}
	r := mux.NewRouter()
	httpinterface.RouterInit(r)
	loggedHandler := handlers.LoggingHandler(os.Stdout, r)

	srv := &http.Server{
		Handler: loggedHandler,
		Addr:    ":4444",
	}
	log.Printf("listening on %s", srv.Addr)
	log.Fatal(srv.ListenAndServe())
}
