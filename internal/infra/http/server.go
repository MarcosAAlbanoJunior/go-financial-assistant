package httpserver

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"time"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain/ports"
	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/usecase"
)

type ServerConfig struct {
	Port            int
	OwnerPhone      string
	AllowedNumbers  map[string]struct{}
	EvolutionAPIURL string
	AdminSecret     string
}

type Server struct {
	http    *http.Server
	handler *webhookHandler
}

func NewServer(
	cfg ServerConfig,
	analyzeExpense usecase.ExpenseAnalyzer,
	csvExporter usecase.CSVExporter,
	messenger ports.Messenger,
	qrProvider QRProvider,
	logger *slog.Logger,
) *Server {
	logger.Info("iniciando servidor HTTP", "port", cfg.Port)

	handler := newWebhookHandler(cfg, analyzeExpense, csvExporter, messenger, logger)
	qrHandler := &qrcodeHandler{secret: cfg.AdminSecret, qrProvider: qrProvider}
	qrLimiter := newIPRateLimiter(10, time.Minute)

	evolutionHost := extractHost(cfg.EvolutionAPIURL)

	mux := http.NewServeMux()
	mux.Handle("/webhook", webhookSourceMiddleware(evolutionHost, logger, http.HandlerFunc(handler.Handle)))
	mux.Handle("/admin/qrcode", adminRateLimitMiddleware(qrLimiter, http.HandlerFunc(qrHandler.Handle)))
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

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
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return s.http.Shutdown(shutdownCtx)
	}
}

func extractHost(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil || u.Hostname() == "" {
		return rawURL
	}
	return u.Hostname()
}
