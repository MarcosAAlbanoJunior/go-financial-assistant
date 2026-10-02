package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	_ "time/tzdata" // fusos horários embutidos: a imagem não precisa de tzdata do sistema

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/app"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	if err := app.Run(ctx, cancel, logger); err != nil {
		logger.Error("o app encerrou com erro", "error", err)
		cancel()
		os.Exit(1)
	}
}
