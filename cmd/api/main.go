// Gopedia Fuego HTTP API (ingest + search via Python subprocess).
package main

import (
	"log/slog"
	"os"

	"gopedia/internal/api"
)

func main() {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo})))

	addr := os.Getenv("GOPEDIA_HTTP_ADDR")
	if addr == "" {
		addr = "127.0.0.1:8787"
	}
	slog.Info("starting Gopedia API", "addr", addr)
	if err := api.Run(addr); err != nil {
		slog.Error("server exit", "err", err)
		os.Exit(1)
	}
}
