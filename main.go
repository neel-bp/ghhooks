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

	"ghhooks.com/hook/core"
	"ghhooks.com/hook/httpinterface"
	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
)

// TODO: flag to control if github commit badge should be updated or not
// TODO: gracefull shutdown that waits for server to shutdown and also waits for current build to finish
//===========
// NOTE: CURRENT FINDINGS about graceful shutdown
// what i want is, whenever sigint signal is sent, it should drain all queued jobs but let the current processing
// job continue instead it along with draining ends the current goroutine even though i am listening for sigint signal
// to test it, use drainall function without sigint signal and it will work fine alternatively, comment draining of channel in drain function
// it will exit the current process and start another job in queue immediately
//===========
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
		sigc := make(chan os.Signal, 1)
		signal.Notify(sigc, os.Interrupt)
		<-sigc
		fmt.Println("gracefully shutting down")
		// core.Queues.DrainAll()

		if err := srv.Shutdown(context.Background()); err != nil {
			l.Printf("HTTP server shurdown error: %v\n", err)
		}
		httpServerCloseChan <- struct{}{}
	}()

	if err := srv.ListenAndServe(); err != http.ErrServerClosed {
		l.Fatalf("http server listen and serve error %v\n", err)
	}

	<-httpServerCloseChan
	core.Queues.DrainAll()
	wg.Wait()

	fmt.Println("done")

}
