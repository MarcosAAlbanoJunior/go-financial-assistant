package settings

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Row é uma configuração como está no banco; segredos vêm cifrados.
type Row struct {
	Key    string
	Value  string
	Secret bool
}

// Store é o armazenamento das configurações editadas no dashboard.
type Store interface {
	LoadSettings(ctx context.Context) ([]Row, error)
	SaveSetting(ctx context.Context, row Row) error
	DeleteSetting(ctx context.Context, key string) error
}

var (
	ErrUnknownKey      = errors.New("configuração desconhecida")
	ErrNoEncryptionKey = errors.New("defina APP_SECRET_KEY no ambiente (mínimo 16 caracteres) para guardar segredos aqui")
)

// ValidationError é um valor recusado; a mensagem é para a pessoa e nada foi gravado.
type ValidationError struct{ Err error }

func (e *ValidationError) Error() string { return e.Err.Error() }
func (e *ValidationError) Unwrap() error { return e.Err }

type Source string

const (
	SourceDB      Source = "db"
	SourceEnv     Source = "env"
	SourceDefault Source = "default"
)

// Service resolve cada configuração (banco > ambiente > padrão) e avisa quando algo muda.
type Service struct {
	store  Store
	cipher *Cipher
	logger *slog.Logger
	env    func(string) (string, bool)

	mu        sync.RWMutex
	db        map[string]string // valores já decifrados
	startup   map[string]string // valores efetivos quando o app subiu (para saber o que pede reinício)
	listeners []func()
}

func NewService(store Store, cipher *Cipher, logger *slog.Logger) *Service {
	return &Service{store: store, cipher: cipher, logger: logger, env: os.LookupEnv, db: map[string]string{}, startup: map[string]string{}}
}

// LoadOverrides lê o que foi salvo no banco, decifrando os segredos. Segredo ilegível é ignorado (com aviso).
func LoadOverrides(ctx context.Context, store Store, cipher *Cipher, logger *slog.Logger) (map[string]string, error) {
	rows, err := store.LoadSettings(ctx)
	if err != nil {
		return nil, err
	}
	out := make(map[string]string, len(rows))
	for _, r := range rows {
		if _, ok := Lookup(r.Key); !ok {
			continue
		}
		v := r.Value
		if r.Secret {
			if cipher == nil {
				logger.Warn("segredo salvo ignorado: APP_SECRET_KEY não definida", "key", r.Key)
				continue
			}
			if v, err = cipher.Decrypt(r.Key, r.Value); err != nil {
				logger.Warn("segredo salvo ignorado", "key", r.Key, "error", err)
				continue
			}
		}
		out[r.Key] = v
	}
	return out, nil
}

// Load carrega os valores do banco e fixa o "como o app subiu".
func (s *Service) Load(ctx context.Context) error {
	values, err := LoadOverrides(ctx, s.store, s.cipher, s.logger)
	if err != nil {
		return err
	}
	s.mu.Lock()
	s.db = values
	s.startup = map[string]string{}
	for _, d := range Defs {
		s.startup[d.Key] = s.resolveLocked(d)
	}
	s.mu.Unlock()
	return nil
}

func (s *Service) resolveLocked(d Def) string {
	if v, ok := s.db[d.Key]; ok {
		return v
	}
	if v, ok := s.env(d.Env); ok && strings.TrimSpace(v) != "" {
		return v
	}
	return d.Default
}

func (s *Service) sourceLocked(d Def) Source {
	if _, ok := s.db[d.Key]; ok {
		return SourceDB
	}
	if v, ok := s.env(d.Env); ok && strings.TrimSpace(v) != "" {
		return SourceEnv
	}
	return SourceDefault
}

// Get devolve o valor efetivo (banco > ambiente > padrão).
func (s *Service) Get(key string) string {
	d, ok := Lookup(key)
	if !ok {
		return ""
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.resolveLocked(d)
}

// OnChange registra quem precisa saber quando uma configuração mudar.
func (s *Service) OnChange(fn func()) {
	s.mu.Lock()
	s.listeners = append(s.listeners, fn)
	s.mu.Unlock()
}

func (s *Service) notify() {
	s.mu.RLock()
	ls := append([]func(){}, s.listeners...)
	s.mu.RUnlock()
	for _, fn := range ls {
		fn()
	}
}

// EncryptionEnabled diz se segredos podem ser guardados aqui.
func (s *Service) EncryptionEnabled() bool { return s.cipher != nil }

// Set grava valores já validados. Valida todos antes de gravar algum.
func (s *Service) Set(ctx context.Context, values map[string]string) ([]string, error) {
	defs := make(map[string]Def, len(values))
	for k, v := range values {
		d, ok := Lookup(k)
		if !ok || d.Internal {
			return nil, fmt.Errorf("%w: %s", ErrUnknownKey, k)
		}
		if d.Kind == KindSecret && s.cipher == nil {
			return nil, ErrNoEncryptionKey
		}
		if err := d.Check(v); err != nil {
			return nil, &ValidationError{err}
		}
		defs[k] = d
	}

	var changed []string
	for k, v := range values {
		d := defs[k]
		row := Row{Key: k, Value: v, Secret: d.Kind == KindSecret}
		if row.Secret {
			enc, err := s.cipher.Encrypt(k, v)
			if err != nil {
				return nil, fmt.Errorf("erro ao cifrar %s", k)
			}
			row.Value = enc
		}
		if err := s.store.SaveSetting(ctx, row); err != nil {
			return changed, err
		}
		s.mu.Lock()
		s.db[k] = v
		s.mu.Unlock()
		changed = append(changed, k)
	}
	s.notify()
	return changed, nil
}

// Reset apaga o valor salvo: volta a valer o ambiente (ou o padrão).
func (s *Service) Reset(ctx context.Context, key string) error {
	if d, ok := Lookup(key); !ok || d.Internal {
		return fmt.Errorf("%w: %s", ErrUnknownKey, key)
	}
	if err := s.store.DeleteSetting(ctx, key); err != nil {
		return err
	}
	s.mu.Lock()
	delete(s.db, key)
	s.mu.Unlock()
	s.notify()
	return nil
}

// Adopt registra valores que outro caminho já gravou no banco (o setup, numa transação) e que já estão valendo: não
// pedem reinício. Avisa quem escuta, como Set.
func (s *Service) Adopt(values map[string]string) {
	s.mu.Lock()
	for k, v := range values {
		s.db[k] = v
		s.startup[k] = v
	}
	s.mu.Unlock()
	s.notify()
}

// Field é uma configuração para a tela. Segredo nunca leva o valor: só se está configurado.
type Field struct {
	Def
	Value          string
	IsSet          bool
	Source         Source
	PendingRestart bool
}

// Fields lista as configurações do canal ativo (as de outro canal ficam de fora).
func (s *Service) Fields(channel string) []Field {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []Field
	for _, d := range Defs {
		if d.Internal || (d.Channel != "" && d.Channel != channel) {
			continue
		}
		v := s.resolveLocked(d)
		f := Field{Def: d, Value: v, IsSet: v != "", Source: s.sourceLocked(d)}
		f.PendingRestart = !d.Live && s.startup[d.Key] != v
		if d.Kind == KindSecret {
			f.Value = ""
		}
		out = append(out, f)
	}
	return out
}

// NeedsRestart diz se alguma configuração que só vale após reiniciar foi alterada.
func (s *Service) NeedsRestart(channel string) bool {
	for _, f := range s.Fields(channel) {
		if f.PendingRestart {
			return true
		}
	}
	return false
}

// --- leitura tipada, para quem consome (sempre o valor atual) ---

func (s *Service) Bool(key string) bool {
	b, _ := strconv.ParseBool(strings.TrimSpace(s.Get(key)))
	return b
}

func (s *Service) Int(key string) int {
	n, _ := strconv.Atoi(strings.TrimSpace(s.Get(key)))
	return n
}

func (s *Service) List(key string) []string { return SplitList(s.Get(key)) }

// Digest devolve a agenda do resumo semanal.
func (s *Service) Digest() (enabled bool, weekday time.Weekday, hour int, loc *time.Location) {
	loc, err := time.LoadLocation(s.Get("DIGEST_TIMEZONE"))
	if err != nil {
		loc, _ = time.LoadLocation("America/Sao_Paulo")
		if loc == nil {
			loc = time.UTC
		}
	}
	weekday = time.Monday
	for i, name := range weekdays {
		if name == strings.ToLower(s.Get("DIGEST_WEEKDAY")) {
			weekday = time.Weekday((i + 1) % 7)
		}
	}
	return s.Bool("DIGEST_ENABLED"), weekday, s.Int("DIGEST_HOUR"), loc
}

// Location é o fuso configurado.
func (s *Service) Location() *time.Location {
	_, _, _, loc := s.Digest()
	return loc
}

// SyncSettings reúne o que a sincronização do Open Finance usa.
type SyncSettings struct {
	Interval     time.Duration
	LookbackDays int
	ItemIDs      []string
	ClientID     string
	ClientSecret string
}

// Configured diz se há credenciais e ao menos um banco.
func (c SyncSettings) Configured() bool {
	return c.ClientID != "" && c.ClientSecret != "" && len(c.ItemIDs) > 0
}

func (s *Service) Sync() SyncSettings {
	return SyncSettings{
		Interval:     time.Duration(max(1, s.Int("SYNC_INTERVAL_HOURS"))) * time.Hour,
		LookbackDays: max(1, s.Int("SYNC_LOOKBACK_DAYS")),
		ItemIDs:      s.List("PLUGGY_ITEM_IDS"),
		ClientID:     strings.TrimSpace(s.Get("PLUGGY_CLIENT_ID")),
		ClientSecret: strings.TrimSpace(s.Get("PLUGGY_CLIENT_SECRET")),
	}
}
