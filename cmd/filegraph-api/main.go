package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	filegraphservice "github.com/thomxsnguyen/mini-distributed-job-api/internal/filegraph/service"
	githubsource "github.com/thomxsnguyen/mini-distributed-job-api/internal/github"
	"github.com/thomxsnguyen/mini-distributed-job-api/internal/httpapi"
)

const (
	defaultAddress       = "127.0.0.1:8080"
	serverShutdownPeriod = 10 * time.Second
)

func main() {
	logger := log.New(os.Stderr, "filegraph-api: ", log.LstdFlags)
	service := filegraphservice.New(
		&githubsource.GitHubClient{Token: os.Getenv("GITHUB_TOKEN")},
		filegraphservice.Options{},
	)
	mux := http.NewServeMux()
	mux.Handle("/api/file-graphs", httpapi.NewFileGraphHandler(service, logger))

	address := os.Getenv("FILEGRAPH_API_ADDR")
	if address == "" {
		address = defaultAddress
	}
	server := newServer(address, mux)
	errorChannel := make(chan error, 1)
	go func() {
		errorChannel <- server.ListenAndServe()
	}()
	logger.Printf("listening on http://%s", address)

	signalContext, stopSignals := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stopSignals()
	select {
	case err := <-errorChannel:
		if !errors.Is(err, http.ErrServerClosed) {
			logger.Fatal(err)
		}
		return
	case <-signalContext.Done():
	}

	shutdownContext, cancelShutdown := context.WithTimeout(context.Background(), serverShutdownPeriod)
	defer cancelShutdown()
	if err := server.Shutdown(shutdownContext); err != nil {
		logger.Printf("shutdown: %v", err)
	}
}

func newServer(address string, handler http.Handler) *http.Server {
	return &http.Server{
		Addr:              address,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      2 * time.Minute,
		IdleTimeout:       30 * time.Second,
	}
}
