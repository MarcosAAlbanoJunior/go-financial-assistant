// Package setup guarda o estado do setup pelo navegador (docs/specs/setup-inicial.md): a senha do dashboard, o rascunho
// do canal e a regra de quando o app está "configurado". Não conhece HTTP nem o Telegram: quem conversa com eles é a API.
package setup

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/auth"
	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/settings"
)

const (
	// MinTokenLen é o tamanho mínimo do SETUP_TOKEN (o make init gera 32).
	MinTokenLen = 24
	// MinPasswordLen é o mesmo mínimo da DASHBOARD_PASSWORD.
	MinPasswordLen = 12
	// draftTokenKey amarra o token do bot cifrado no rascunho a este uso (um valor cifrado não serve em outro campo).
	draftTokenKey = "SETUP_TELEGRAM_BOT_TOKEN"
)

var (
	ErrNoToken         = errors.New("defina SETUP_TOKEN no .env e reinicie o app")
	ErrTokenTooShort   = fmt.Errorf("SETUP_TOKEN curto demais: use ao menos %d caracteres (o make init gera um)", MinTokenLen)
	ErrWrongToken      = errors.New("token de setup incorreto")
	ErrNoEncryption    = errors.New("defina APP_SECRET_KEY_FILE (make secret-key) e reinicie: o token do bot é guardado cifrado")
	ErrPasswordShort   = fmt.Errorf("a senha precisa ter ao menos %d caracteres", MinPasswordLen)
	ErrNoPassword      = errors.New("defina a senha do dashboard primeiro")
	ErrNoBot           = errors.New("cole o token do bot primeiro")
	ErrNoCandidate     = errors.New("ainda não recebemos a sua mensagem: mande /start ao bot")
	ErrNotAccepted     = errors.New("confirme que a mensagem é sua primeiro")
	ErrNoChannel       = errors.New("não há canal configurado para manter")
	ErrChannelInEnv    = errors.New("o canal já está definido no .env; para trocar, use SETUP_REOPEN")
	ErrAlreadyComplete = errors.New("o setup já foi concluído")
	ErrClosed          = errors.New("setup indisponível")
)

// Owner é o dono do dashboard como está no banco.
type Owner struct {
	PasswordHash string
	CompletedAt  *time.Time
	ReopenDone   bool
}

// Candidate é quem mandou mensagem ao bot durante o setup (ainda não confirmado).
type Candidate struct {
	ID       int64
	Name     string
	Username string
}

// Draft é o rascunho do setup. Nada daqui vale como configuração até concluir.
type Draft struct {
	PasswordHash      string
	TelegramToken     string // no Store, cifrado; no Service, já decifrado
	TelegramBot       string
	TelegramOffset    int64
	Candidate         *Candidate
	CandidateAccepted bool
}

// Completion é o que a conclusão grava numa transação só.
type Completion struct {
	Rows         []settings.Row // canal (vazio ao manter o canal atual)
	PasswordHash string         // vazio mantém o atual
	Reopen       bool
	// Expected é o dono como estava ao começar: se outra conclusão gravou antes, a transação não grava nada.
	Expected Owner
}

// Store guarda o dono e o rascunho.
type Store interface {
	LoadOwner(ctx context.Context) (Owner, error)
	LoadDraft(ctx context.Context) (Draft, error)
	SaveDraft(ctx context.Context, d Draft) error
	// Complete grava as configurações, o hash e a marca de concluído e apaga o rascunho, tudo ou nada, e devolve quando
	// concluiu. Devolve nil (sem gravar) se o dono não está mais como Expected.
	Complete(ctx context.Context, c Completion) (*time.Time, error)
	// ClearReopen libera uma próxima reabertura (SETUP_REOPEN foi desligado).
	ClearReopen(ctx context.Context) error
}

// Env é o que vem do ambiente.
type Env struct {
	Token       string // SETUP_TOKEN
	Reopen      bool   // SETUP_REOPEN
	EnvPassword string // DASHBOARD_PASSWORD
	// ChannelReady diz se há canal configurado no ambiente (ou salvo antes). É lido a cada uso.
	ChannelReady func() bool
}

// Activation é o canal que a conclusão manda ligar. Keep = manter o canal que já está configurado.
type Activation struct {
	Keep   bool
	Token  string
	ChatID int64
	Offset int64
}

// Service resolve o estado do setup. As operações que gravam são serializadas: duas conclusões ao mesmo tempo, uma grava
// e a outra recebe ErrAlreadyComplete.
type Service struct {
	store  Store
	cipher *settings.Cipher
	env    Env
	logger *slog.Logger

	op sync.Mutex // uma operação de gravação por vez

	mu           sync.RWMutex
	owner        Owner
	channelSaved bool // o setup gravou um canal neste processo
	storeErr     error
	envPassword  [sha256.Size]byte
}

func NewService(store Store, cipher *settings.Cipher, env Env, logger *slog.Logger) *Service {
	if env.ChannelReady == nil {
		env.ChannelReady = func() bool { return false }
	}
	return &Service{store: store, cipher: cipher, env: env, logger: logger, envPassword: sha256.Sum256([]byte(env.EnvPassword))}
}

// Load lê o dono. Sem as tabelas (migration 019 não aplicada) o setup não consegue gravar, mas quem já tem tudo no
// ambiente segue normal.
func (s *Service) Load(ctx context.Context) error {
	owner, err := s.store.LoadOwner(ctx)
	s.mu.Lock()
	s.owner, s.storeErr = owner, err
	s.mu.Unlock()
	if err != nil {
		return err
	}
	if !s.env.Reopen && owner.ReopenDone {
		if err := s.store.ClearReopen(ctx); err != nil {
			return err
		}
		s.mu.Lock()
		s.owner.ReopenDone = false
		s.mu.Unlock()
	}
	if s.env.Reopen && owner.ReopenDone {
		s.logger.Warn("SETUP_REOPEN continua ligado, mas o setup já foi refeito: remova SETUP_REOPEN do .env")
	}
	return nil
}

// --- regras ---

// Open diz se o setup está aberto: o app não está configurado, ou SETUP_REOPEN pediu para refazer. Aberto, o dashboard
// só serve o setup.
func (s *Service) Open() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.openLocked()
}

func (s *Service) openLocked() bool { return s.reopenLocked() || !s.configuredLocked() }

// Reopen diz se o setup foi reaberto por SETUP_REOPEN (e ainda não refeito).
func (s *Service) Reopen() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.reopenLocked()
}

func (s *Service) reopenLocked() bool { return s.env.Reopen && !s.owner.ReopenDone }

// configuredLocked: configurado = tem senha (banco, com o setup concluído, ou ambiente) e tem canal (ambiente ou setup).
func (s *Service) configuredLocked() bool {
	return s.hasPasswordLocked() && s.channelLocked()
}

func (s *Service) hasPasswordLocked() bool {
	return s.ownerPasswordLocked() || s.env.EnvPassword != ""
}

// ownerPasswordLocked: a senha do banco só vale com o setup concluído.
func (s *Service) ownerPasswordLocked() bool {
	return s.owner.CompletedAt != nil && s.owner.PasswordHash != ""
}

func (s *Service) channelLocked() bool { return s.channelSaved || s.env.ChannelReady() }

// CheckPassword confere a senha do login: a do banco (hash argon2id), se o setup a definiu, senão a do ambiente. Com o
// setup aberto, nenhuma senha entra.
func (s *Service) CheckPassword(candidate string) bool {
	s.mu.RLock()
	open, hash, fromDB := s.openLocked(), s.owner.PasswordHash, s.ownerPasswordLocked()
	s.mu.RUnlock()
	switch {
	case open:
		return false
	case fromDB:
		return auth.VerifyPassword(hash, candidate)
	case s.env.EnvPassword != "":
		h := sha256.Sum256([]byte(candidate))
		return subtle.ConstantTimeCompare(h[:], s.envPassword[:]) == 1
	}
	return false
}

// CheckToken confere o token de setup em tempo constante (via hash, para não vazar nem o tamanho).
func (s *Service) CheckToken(candidate string) error {
	if err := s.TokenProblem(); err != nil {
		return err
	}
	want, got := sha256.Sum256([]byte(s.env.Token)), sha256.Sum256([]byte(candidate))
	if subtle.ConstantTimeCompare(want[:], got[:]) != 1 {
		return ErrWrongToken
	}
	return nil
}

// TokenProblem diz o que impede o setup de começar (sem token, token curto, sem chave mestra, banco sem a migration).
func (s *Service) TokenProblem() error {
	switch {
	case s.env.Token == "":
		return ErrNoToken
	case utf8.RuneCountInString(s.env.Token) < MinTokenLen:
		return ErrTokenTooShort
	case s.cipher == nil:
		return ErrNoEncryption
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.storeErr != nil {
		return fmt.Errorf("banco sem as tabelas do setup: aplique a migration 019 (veja o README)")
	}
	return nil
}

// Status é o que a tela precisa saber dos passos.
type Status struct {
	Reopen          bool
	PasswordFromEnv bool // DASHBOARD_PASSWORD definida: o passo pode ser pulado
	PasswordDraft   bool // senha nova salva no rascunho
	PasswordCurrent bool // reaberto, com a senha de antes ainda valendo
	ChannelInEnv    bool // há canal configurado (ambiente ou setup anterior): pode manter
	Draft           Draft
}

// PasswordReady diz se a conclusão tem senha: nova, do ambiente ou a de antes (reaberto).
func (st Status) PasswordReady() bool {
	return st.PasswordDraft || st.PasswordFromEnv || st.PasswordCurrent
}

func (s *Service) Status(ctx context.Context) (Status, error) {
	d, err := s.loadDraft(ctx)
	if err != nil {
		return Status{}, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return Status{
		Reopen:          s.reopenLocked(),
		PasswordFromEnv: s.env.EnvPassword != "",
		PasswordDraft:   d.PasswordHash != "",
		PasswordCurrent: s.reopenLocked() && s.owner.PasswordHash != "",
		ChannelInEnv:    s.channelLocked(),
		Draft:           d,
	}, nil
}

func (s *Service) loadDraft(ctx context.Context) (Draft, error) {
	d, err := s.store.LoadDraft(ctx)
	if err != nil {
		return Draft{}, err
	}
	if d.TelegramToken != "" {
		if s.cipher == nil {
			return Draft{}, ErrNoEncryption
		}
		if d.TelegramToken, err = s.cipher.Decrypt(draftTokenKey, d.TelegramToken); err != nil {
			// Chave mestra trocada no meio do setup: o token salvo não serve mais; a pessoa cola de novo.
			s.logger.Warn("token do bot no rascunho ilegível; descartado", "error", err)
			d.TelegramToken, d.TelegramBot, d.Candidate, d.CandidateAccepted = "", "", nil, false
		}
	}
	return d, nil
}

// update lê o rascunho, aplica fn e grava, com o setup aberto e uma operação por vez.
func (s *Service) update(ctx context.Context, fn func(d *Draft) error) (Draft, error) {
	s.op.Lock()
	defer s.op.Unlock()
	if !s.Open() {
		return Draft{}, ErrClosed
	}
	d, err := s.loadDraft(ctx)
	if err != nil {
		return Draft{}, err
	}
	if err := fn(&d); err != nil {
		return Draft{}, err
	}
	stored := d
	if stored.TelegramToken != "" {
		if s.cipher == nil {
			return Draft{}, ErrNoEncryption
		}
		if stored.TelegramToken, err = s.cipher.Encrypt(draftTokenKey, d.TelegramToken); err != nil {
			return Draft{}, fmt.Errorf("erro ao cifrar o token do bot: %w", err)
		}
	}
	if err := s.store.SaveDraft(ctx, stored); err != nil {
		return Draft{}, err
	}
	return d, nil
}

// --- passos ---

// SetPassword guarda o hash da senha nova no rascunho. Ela só passa a valer para entrar quando o setup terminar.
func (s *Service) SetPassword(ctx context.Context, password string) error {
	if utf8.RuneCountInString(password) < MinPasswordLen {
		return ErrPasswordShort
	}
	hash, err := auth.HashPassword(password)
	if err != nil {
		return err
	}
	_, err = s.update(ctx, func(d *Draft) error {
		d.PasswordHash = hash
		return nil
	})
	return err
}

// SetBot guarda o token (já validado com getMe) e recomeça a espera do /start a partir de offset: trocar de bot descarta
// quem tinha mandado mensagem ao anterior.
func (s *Service) SetBot(ctx context.Context, token, bot string, offset int64) error {
	_, err := s.update(ctx, func(d *Draft) error {
		if s.channelInEnvUnlessReopen() {
			return ErrChannelInEnv
		}
		d.TelegramToken, d.TelegramBot, d.TelegramOffset = token, bot, offset
		d.Candidate, d.CandidateAccepted = nil, false
		return nil
	})
	return err
}

// channelInEnvUnlessReopen: fora da reabertura, um canal que já existe não é trocado pelo setup (seriam dois bots).
func (s *Service) channelInEnvUnlessReopen() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return !s.reopenLocked() && s.channelLocked()
}

// SetCandidate guarda quem mandou a primeira mensagem privada e avança o offset para depois dela. Se já há alguém
// esperando confirmação, não troca.
func (s *Service) SetCandidate(ctx context.Context, c Candidate, offset int64) (Draft, error) {
	return s.update(ctx, func(d *Draft) error {
		if d.TelegramToken == "" {
			return ErrNoBot
		}
		if d.Candidate == nil {
			d.Candidate, d.CandidateAccepted = &c, false
		}
		d.TelegramOffset = max(d.TelegramOffset, offset)
		return nil
	})
}

// AdvanceOffset avança a leitura do Telegram sem candidato (só mensagens de grupo ou de canal).
func (s *Service) AdvanceOffset(ctx context.Context, offset int64) error {
	_, err := s.update(ctx, func(d *Draft) error {
		d.TelegramOffset = max(d.TelegramOffset, offset)
		return nil
	})
	return err
}

// RejectCandidate é o "não sou eu": descarta quem mandou e continua esperando (o offset já passou dessa mensagem).
func (s *Service) RejectCandidate(ctx context.Context) error {
	_, err := s.update(ctx, func(d *Draft) error {
		d.Candidate, d.CandidateAccepted = nil, false
		return nil
	})
	return err
}

// AcceptCandidate é o "sim, sou eu": o próximo passo é o código enviado a esse chat.
func (s *Service) AcceptCandidate(ctx context.Context) (Draft, error) {
	return s.update(ctx, func(d *Draft) error {
		if d.TelegramToken == "" {
			return ErrNoBot
		}
		if d.Candidate == nil {
			return ErrNoCandidate
		}
		d.CandidateAccepted = true
		return nil
	})
}

// CompleteTelegram conclui com o Telegram do rascunho: grava canal, senha e "concluído" numa transação. Os valores vêm
// só do rascunho (o código já foi conferido por quem chama), nunca da requisição.
func (s *Service) CompleteTelegram(ctx context.Context) (Activation, error) {
	var act Activation
	err := s.complete(ctx, func(st Status, c *Completion) error {
		d := st.Draft
		switch {
		case d.TelegramToken == "":
			return ErrNoBot
		case d.Candidate == nil:
			return ErrNoCandidate
		case !d.CandidateAccepted:
			return ErrNotAccepted
		}
		token, err := s.cipher.Encrypt("TELEGRAM_BOT_TOKEN", d.TelegramToken)
		if err != nil {
			return fmt.Errorf("erro ao cifrar o token do bot: %w", err)
		}
		c.Rows = []settings.Row{
			{Key: "CHANNEL", Value: "telegram"},
			{Key: "TELEGRAM_BOT_TOKEN", Value: token, Secret: true},
			{Key: "TELEGRAM_CHAT_ID", Value: strconv.FormatInt(d.Candidate.ID, 10)},
		}
		act = Activation{Token: d.TelegramToken, ChatID: d.Candidate.ID, Offset: d.TelegramOffset}
		return nil
	})
	return act, err
}

// CompleteKeep conclui mantendo o canal que já está configurado (no ambiente ou de um setup anterior).
func (s *Service) CompleteKeep(ctx context.Context) error {
	return s.complete(ctx, func(st Status, _ *Completion) error {
		if !st.ChannelInEnv {
			return ErrNoChannel
		}
		return nil
	})
}

func (s *Service) complete(ctx context.Context, build func(Status, *Completion) error) error {
	s.op.Lock()
	defer s.op.Unlock()
	if !s.Open() {
		return ErrAlreadyComplete
	}
	if s.cipher == nil {
		return ErrNoEncryption
	}
	st, err := s.Status(ctx)
	if err != nil {
		return err
	}
	if !st.PasswordReady() {
		return ErrNoPassword
	}
	s.mu.RLock()
	c := Completion{PasswordHash: st.Draft.PasswordHash, Reopen: s.reopenLocked(), Expected: s.owner}
	s.mu.RUnlock()
	if err := build(st, &c); err != nil {
		return err
	}
	at, err := s.store.Complete(ctx, c)
	if err != nil {
		return err
	}
	if at == nil {
		return ErrAlreadyComplete
	}

	s.mu.Lock()
	s.owner.CompletedAt = at
	if c.PasswordHash != "" {
		s.owner.PasswordHash = c.PasswordHash
	}
	if c.Reopen {
		s.owner.ReopenDone = true
	}
	if len(c.Rows) > 0 {
		s.channelSaved = true
	}
	s.mu.Unlock()
	if c.Reopen {
		s.logger.Warn("setup refeito: remova SETUP_REOPEN do .env (enquanto estiver lá, nada mais acontece)")
	}
	return nil
}
