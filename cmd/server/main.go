package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/cgopalan/s3fileviewer/internal/config"
	"github.com/cgopalan/s3fileviewer/internal/duckdb"
	"github.com/cgopalan/s3fileviewer/internal/handlers"
	s3client "github.com/cgopalan/s3fileviewer/internal/s3"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	ctx := context.Background()

	s3, err := s3client.NewClient(ctx, cfg.AWSRegion)
	if err != nil {
		log.Fatalf("s3 client: %v", err)
	}

	db, err := duckdb.NewPool(cfg.MaxQueryRows, cfg.QueryTimeoutSec, cfg.AWSRegion, s3)
	if err != nil {
		log.Fatalf("duckdb: %v", err)
	}
	defer db.Close()

	h, err := handlers.New(cfg, s3, db)
	if err != nil {
		log.Fatalf("handlers: %v", err)
	}

	srv := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      h.Routes(),
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		log.Printf("listening on http://localhost:%s", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("shutdown: %v", err)
	}
}
