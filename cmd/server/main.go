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

	"patrol-platform/internal/config"
	"patrol-platform/internal/service"
	"patrol-platform/internal/store"
	"patrol-platform/internal/transport"
	"patrol-platform/internal/worker"
)

func main() {
	cfgPath := os.Getenv("PATROL_CONFIG")
	if cfgPath == "" {
		cfgPath = "config.json"
	}
	cfg, err := config.Load(cfgPath)
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	st := store.New()
	clock := service.RealClock{}
	gridSvc := service.NewGridService(st)
	taskSvc := service.NewTaskService(st, cfg, clock)
	alarmSvc := service.NewAlarmService(st, cfg, clock)
	terminalSvc := service.NewTerminalService(st, cfg, taskSvc, clock)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	wk := worker.New(st, taskSvc, alarmSvc, cfg)
	go wk.Run(ctx)

	handler := transport.NewRouter(gridSvc, taskSvc, alarmSvc, terminalSvc)
	server := &http.Server{
		Addr:         cfg.HTTPAddr,
		Handler:      handler,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh
		log.Println("shutting down...")
		cancel()
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer shutdownCancel()
		_ = server.Shutdown(shutdownCtx)
	}()

	log.Printf("patrol-platform listening on %s", cfg.HTTPAddr)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatalf("server error: %v", err)
	}
}
