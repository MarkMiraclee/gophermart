package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/MarkMiraclee/gophermart/internal/accrual"
	"github.com/MarkMiraclee/gophermart/internal/config"
	"github.com/MarkMiraclee/gophermart/internal/handlers"
	"github.com/MarkMiraclee/gophermart/internal/storage"
	"github.com/sirupsen/logrus"
)

func main() {
	log := logrus.New()
	log.SetFormatter(&logrus.JSONFormatter{})
	log.SetOutput(os.Stdout)
	log.SetLevel(logrus.InfoLevel)

	cfg, err := config.New()
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	db, err := storage.NewPostgresStorage(ctx, cfg.DatabaseURI)
	if err != nil {
		log.Fatalf("failed to initialize storage: %v", err)
	}
	defer db.Close()

	accrualClient := accrual.NewClient(cfg.AccrualSystemAddress, db, log)
	go accrualClient.Start(ctx)

	api := handlers.NewAPI(db, log, cfg.JWTSecret)
	router := handlers.NewRouter(api)

	server := &http.Server{
		Addr:    cfg.RunAddress,
		Handler: router,
	}

	go func() {
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("listen: %s\n", err)
		}
	}()
	log.Infof("server started on %s", cfg.RunAddress)

	<-ctx.Done()

	log.Info("shutting down server gracefully")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("server shutdown failed: %+v", err)
	}

	log.Info("server exited properly")
}
