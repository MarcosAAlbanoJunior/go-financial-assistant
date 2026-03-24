package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/config"
	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/infra/db"
	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/infra/evolution"
	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/infra/gemini"
	httpserver "github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/infra/http"
	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/usecase"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	cfg, err := config.Load()
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	ctx, cancel := signal.NotifyContext(context.Background(),
		syscall.SIGINT, syscall.SIGTERM,
	)
	defer cancel()

	postgresDB, err := db.NewPostgres(ctx, cfg.DatabaseURL)
	if err != nil {
		slog.Error("failed to connect to postgres", "error", err)
		os.Exit(1)
	}
	defer postgresDB.Close()

	geminiClient, err := gemini.NewClient(ctx, cfg.GeminiAPIKey)
	if err != nil {
		slog.Error("failed to create gemini client", "error", err)
		os.Exit(1)
	}
	defer geminiClient.Close()

	expenseRepo := db.NewExpenseRepository(postgresDB)
	installmentRepo := db.NewInstallmentRepository(postgresDB)
	recurringRepo := db.NewRecurringExpenseRepository(postgresDB)

	analyzeExpense := usecase.NewAnalyzeExpense(expenseRepo, installmentRepo, recurringRepo, geminiClient, logger)

	go func() {
		for {
			now := time.Now().UTC()
			next := time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, time.UTC)
			select {
			case <-ctx.Done():
				return
			case <-time.After(time.Until(next)):
				if err := analyzeExpense.GenerateRecurringExpenses(ctx); err != nil {
					slog.Error("erro ao gerar despesas recorrentes", "error", err)
				}
			}
		}
	}()

	evolutionClient := evolution.NewClient(cfg.EvolutionAPIURL, cfg.EvolutionInstance, cfg.EvolutionAPIKey)

	server := httpserver.NewServer(
		httpserver.ServerConfig{
			Port:           cfg.Port,
			OwnerPhone:     cfg.OwnerPhone,
			AllowedNumbers: cfg.AllowedNumbers,
		},
		analyzeExpense,
		evolutionClient,
		logger,
	)

	slog.Info("starting go-financial-assistant", "port", cfg.Port)

	if err := server.Start(ctx); err != nil {
		slog.Error("server error", "error", err)
		os.Exit(1)
	}

	slog.Info("server stopped gracefully")
}
