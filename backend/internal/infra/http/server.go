package httpserver

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/usecase/ledger"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain/ports"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/chat"
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

	// coach é opcional; coachPaid é a declaração de que a chave do Gemini é de um projeto com faturamento.
	coach     ports.Coach
	coachPaid bool

	// syncer é opcional (nil sem Open Finance); alimenta o botão de sincronizar do painel.
	syncer chat.Syncer

	// settings liga a página de configurações (nil = desligada).
	settings *SettingsDeps

	// factor liga o código no chat ao login e às confirmações (nil = só senha).
	factor *SecondFactor

	// setup liga o setup pelo navegador (nil = desligado).
	setup *SetupDeps

	mu     sync.Mutex
	runCtx context.Context // definido em Start: rotas montadas depois (canal ligado pelo setup) já sobem as limpezas
}

// SetSyncer liga a sincronização do Open Finance à API do dashboard (chame antes de MountAPI).
func (s *Server) SetSyncer(syncer chat.Syncer) { s.syncer = syncer }

// SetCoach liga o Coach com IA da API do dashboard. Sem plano pago declarado, ele fica bloqueado.
func (s *Server) SetCoach(coach ports.Coach, paidPlan bool) {
	s.coach, s.coachPaid = coach, paidPlan
}

// NewServer expõe apenas /health; as rotas de cada canal são adicionadas por Mount*.
func NewServer(port int, logger *slog.Logger) *Server {
	logger.Info("iniciando servidor HTTP", "port", port)

	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	s := &Server{mux: mux, logger: logger}
	s.http = &http.Server{Addr: fmt.Sprintf(":%d", port), Handler: http.HandlerFunc(s.serve)}
	return s
}

func (s *Server) serve(w http.ResponseWriter, r *http.Request) {
	if s.setupGate(w, r) {
		return
	}
	s.mux.ServeHTTP(w, r)
}

// MountWhatsApp registra o webhook da Evolution API e o endpoint de QR code.
// syncer pode ser nil quando o Open Finance não está configurado.
func (s *Server) MountWhatsApp(cfg WhatsAppConfig, client EvolutionClient, analyzeExpense ledger.ExpenseAnalyzer, csvExporter ledger.CSVExporter, syncer chat.Syncer) {
	handler := newWebhookHandler(cfg, client, analyzeExpense, csvExporter, s.logger)
	handler.chat.SetSyncer(syncer)
	qrHandler := &qrcodeHandler{secret: cfg.AdminSecret, qrProvider: client}
	qrLimiter := newIPRateLimiter(10, time.Minute)

	s.mux.Handle("/webhook", webhookSourceMiddleware(extractHost(cfg.EvolutionAPIURL), s.logger, http.HandlerFunc(handler.Handle)))
	s.mux.Handle("/admin/qrcode", adminRateLimitMiddleware(qrLimiter, http.HandlerFunc(qrHandler.Handle)))
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.runCtx != nil {
		go handler.startCleanup(s.runCtx)
		return
	}
	s.cleanups = append(s.cleanups, handler.startCleanup)
}

func (s *Server) Start(ctx context.Context) error {
	s.mu.Lock()
	s.runCtx = ctx
	for _, cleanup := range s.cleanups {
		go cleanup(ctx)
	}
	s.mu.Unlock()

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
