package httpserver

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain/ports"
	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/usecase"
)

type ServerConfig struct {
	Port           int
	OwnerPhone     string
	AllowedNumbers map[string]struct{}
}

type Server struct {
	http    *http.Server
	handler *webhookHandler
}

func NewServer(cfg ServerConfig, analyzeExpense usecase.ExpenseAnalyzer, messenger ports.Messenger, logger *slog.Logger) *Server {
	logger.Info("iniciando servidor HTTP", "port", cfg.Port)

	handler := newWebhookHandler(cfg, analyzeExpense, messenger, logger)

	mux := http.NewServeMux()
	mux.HandleFunc("/webhook", handler.Handle)

	return &Server{
		http: &http.Server{
			Addr:    fmt.Sprintf(":%d", cfg.Port),
			Handler: mux,
		},
		handler: handler,
	}
}

func (s *Server) Start(ctx context.Context) error {
	go s.handler.startCleanup(ctx)

	errCh := make(chan error, 1)

	go func() {
		if err := s.http.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		return s.http.Shutdown(context.Background())
	}
}

