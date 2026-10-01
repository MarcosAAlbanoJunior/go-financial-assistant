package httpserver

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"time"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/usecase"
)

// WhatsAppConfig reúne o que as rotas do canal WhatsApp (Evolution API) precisam.
type WhatsAppConfig struct {
	OwnerPhone      string
	AllowedNumbers  map[string]struct{}
	EvolutionAPIURL string
	AdminSecret     string
}

type Server struct {
	http     *http.Server
	mux      *http.ServeMux
	logger   *slog.Logger
	cleanups []func(context.Context)
}

// NewServer expõe apenas /health; as rotas de cada canal são adicionadas por Mount*.
func NewServer(port int, logger *slog.Logger) *Server {
	logger.Info("iniciando servidor HTTP", "port", port)

	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	return &Server{
		http:   &http.Server{Addr: fmt.Sprintf(":%d", port), Handler: mux},
		mux:    mux,
		logger: logger,
	}
}

// MountWhatsApp registra o webhook da Evolution API e o endpoint de QR code.
func (s *Server) MountWhatsApp(cfg WhatsAppConfig, client EvolutionClient, analyzeExpense usecase.ExpenseAnalyzer, csvExporter usecase.CSVExporter) {
	handler := newWebhookHandler(cfg, client, analyzeExpense, csvExporter, s.logger)
	qrHandler := &qrcodeHandler{secret: cfg.AdminSecret, qrProvider: client}
	qrLimiter := newIPRateLimiter(10, time.Minute)

	s.mux.Handle("/webhook", webhookSourceMiddleware(extractHost(cfg.EvolutionAPIURL), s.logger, http.HandlerFunc(handler.Handle)))
	s.mux.Handle("/admin/qrcode", adminRateLimitMiddleware(qrLimiter, http.HandlerFunc(qrHandler.Handle)))
	s.cleanups = append(s.cleanups, handler.startCleanup)
}

func (s *Server) Start(ctx context.Context) error {
	for _, cleanup := range s.cleanups {
		go cleanup(ctx)
	}

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
