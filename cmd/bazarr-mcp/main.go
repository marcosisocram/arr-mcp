package main

import (
	"context"
	"errors"
	"flag"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/marcosisocram/arr-mcp/internal/bazarr"
)

func main() {
	healthcheck := flag.Bool("healthcheck", false, "check the local server health endpoint")
	flag.Parse()
	if *healthcheck {
		if err := checkHealth(); err != nil {
			os.Exit(1)
		}
		return
	}
	if err := run(); err != nil {
		slog.Error("Bazarr MCP server stopped", "error", err)
		os.Exit(1)
	}
}

func checkHealth() error {
	address := os.Getenv("BAZARR_LISTEN_ADDR")
	if address == "" {
		address = "127.0.0.1:8080"
	}
	_, port, err := net.SplitHostPort(address)
	if err == nil {
		address = net.JoinHostPort("127.0.0.1", port)
	}
	client := http.Client{Timeout: 2 * time.Second}
	response, err := client.Get("http://" + address + "/health")
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return errors.New("health endpoint returned non-200 status")
	}
	return nil
}

func run() error {
	cfg, err := bazarr.LoadConfig()
	if err != nil {
		return err
	}
	httpServer := &http.Server{
		Addr:              cfg.Listen,
		Handler:           bazarr.NewHTTPHandler(bazarr.NewClient(cfg)),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	serveErr := make(chan error, 1)
	go func() { serveErr <- httpServer.ListenAndServe() }()
	slog.Info("Bazarr MCP listening", "address", cfg.Listen, "path", "/mcp/bazarr")
	select {
	case err := <-serveErr:
		if !errors.Is(err, http.ErrServerClosed) {
			return err
		}
	case <-ctx.Done():
	}
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return httpServer.Shutdown(shutdownCtx)
}
