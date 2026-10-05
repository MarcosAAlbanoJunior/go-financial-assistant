package httpserver

import (
	"log/slog"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/auth"
	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/usecase/insights"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/chat"
	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain/ports"
)

// api serve as rotas JSON de leitura do dashboard sob /api.
type api struct {
	reader   ports.DashboardReader
	sessions *sessions
	logger   *slog.Logger
	now      func() time.Time
	coach    *coachService
	insights *insights.Insights
	syncer   chat.Syncer   // nil sem Open Finance
	settings *SettingsDeps // nil sem a página de configurações
	factor   *SecondFactor // nil = só senha

	challenges *auth.Challenges

	confirmLimiter *ipRateLimiter // tentativas de confirmar (senha ou código) as configurações sensíveis
	lastFailNotice atomic.Int64

	setup       *SetupDeps // nil sem o setup pelo navegador
	setupSess   *setupSessions
	setupPollMu sync.RWMutex // consultas ao /start (leitura) x conclusão (escrita)
}

// MountAPI registra a API do dashboard. Tudo, exceto o login (e o setup, que tem a sua sessão), exige sessão.
func (s *Server) MountAPI(password PasswordCheck, reader ports.DashboardReader) error {
	return s.mountAPI(password, reader, time.Now)
}

func (s *Server) mountAPI(password PasswordCheck, reader ports.DashboardReader, now func() time.Time) error {
	sess, err := newSessions(password)
	if err != nil {
		return err
	}
	a := &api{reader: reader, sessions: sess, logger: s.logger, now: now, coach: newCoachService(s.coach, s.coachPaid), insights: insights.NewInsights(reader), syncer: s.syncer, settings: s.settings, factor: s.factor, challenges: auth.NewChallenges(), setup: s.setup}
	if a.setupSess, err = newSetupSessions(); err != nil {
		return err
	}

	// Login com limite apertado contra tentativa de força bruta; o restante, mais folgado.
	loginLimiter := newIPRateLimiter(5, time.Minute)
	apiLimiter := newIPRateLimiter(300, time.Minute)

	rt := routes{
		mux: s.mux,
		protected: func(h http.HandlerFunc) http.Handler {
			return apiLimiter.middleware(sess.require(h))
		},
	}
	a.registerAuth(rt, loginLimiter)
	a.registerSummary(rt)
	a.registerInvestments(rt)
	a.registerBudget(rt)
	a.registerReview(rt)
	a.registerTransactions(rt)
	a.registerAccounts(rt)
	a.registerGoals(rt)
	a.registerCoach(rt)
	a.registerCategorize(rt)
	a.registerSavings(rt)
	a.registerBalances(rt)
	if s.settings != nil {
		a.registerSettings(rt, s.settings)
	}
	if s.setup != nil {
		a.registerSetup(rt)
	}
	return nil
}

// routes agrupa o que cada contexto precisa para registrar as suas rotas: o mux e o envoltório de sessão + limite.
type routes struct {
	mux       *http.ServeMux
	protected func(http.HandlerFunc) http.Handler
}
