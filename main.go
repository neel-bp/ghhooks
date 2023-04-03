package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"

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
	addr := flag.String("addr", ":4444", "address/port pair")
	flag.Parse()

	l := log.New(os.Stdout, "", 0)
	var wg sync.WaitGroup
	err := core.ServerInit(*configFileLocation, l, &wg)
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
		Handler: handler,
		Addr:    *addr,
	}
	log.Printf("listening on %s", srv.Addr)
	// log.Fatal(srv.ListenAndServe())

	// queue drain drains all jobs in queue, but lets the job that is currently underway process without interruption
	httpServerCloseChan := make(chan struct{})
	go func() {
		sigc := make(chan os.Signal, 2)
		signal.Notify(sigc, syscall.SIGINT, syscall.SIGTERM)
		<-sigc
		fmt.Println("gracefully shutting down")
		core.Queues.DrainAll()

		if err := srv.Shutdown(context.Background()); err != nil {
			l.Printf("HTTP server shurdown error: %v\n", err)
		}
		close(httpServerCloseChan)
	}()

	if err := srv.ListenAndServe(); err != http.ErrServerClosed {
		l.Fatalf("http server listen and serve error %v\n", err)
	}

	<-httpServerCloseChan

	fmt.Println("gracefully shutting down")

}
