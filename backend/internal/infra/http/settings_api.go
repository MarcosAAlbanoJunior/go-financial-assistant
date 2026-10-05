package httpserver

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain/ports"
	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/infra/pluggy"
	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/settings"
)

const (
	maxSettingsBody = 16 << 10
	testTimeout     = 20 * time.Second
	maxExamples     = 5
)

// SettingsDeps é o que a página de configurações precisa. Os testes de conexão recebem as credenciais por parâmetro
// (as que estão salvas), para ficarem fora do pacote HTTP.
type SettingsDeps struct {
	Service *settings.Service
	// Channel fixa o canal da página; vazio usa a configuração CHANNEL (que o setup pode mudar com o app rodando).
	Channel      string
	Cleaner      ports.TransferCleaner
	PluggyCheck  func(ctx context.Context, clientID, clientSecret string, itemIDs []string) ([]pluggy.ItemCheck, error)
	TelegramPing func(ctx context.Context, token string) (string, error)
	GeminiPing   func(ctx context.Context, apiKey string) error
	Restart      func() // pede ao processo que encerre; o Docker (restart: unless-stopped) o sobe de novo
	Audit        settings.AuditLog
	// Notify avisa a pessoa no chat (Telegram/WhatsApp) de logins, de algo sensível que mudou e de senha ou código errados. Pode ser nil.
	Notify func(ctx context.Context, text string)
}

func (d *SettingsDeps) channel() string {
	if d.Channel != "" {
		return d.Channel
	}
	return strings.ToLower(d.Service.Get("CHANNEL"))
}

// SetSettings liga a página de configurações à API (chame antes de MountAPI).
func (s *Server) SetSettings(d *SettingsDeps) { s.settings = d }

type fieldJSON struct {
	Key            string   `json:"key"`
	Label          string   `json:"label"`
	Help           string   `json:"help"`
	Kind           string   `json:"kind"`
	Options        []string `json:"options,omitempty"`
	Min            int      `json:"min,omitempty"`
	Max            int      `json:"max,omitempty"`
	Live           bool     `json:"live"`
	Value          string   `json:"value"`
	IsSet          bool     `json:"isSet"`
	Source         string   `json:"source"`
	PendingRestart bool     `json:"pendingRestart"`
	Default        string   `json:"default"`
	Sensitive      bool     `json:"sensitive"`
}

type groupJSON struct {
	ID     string      `json:"id"`
	Title  string      `json:"title"`
	Help   string      `json:"help"`
	Fields []fieldJSON `json:"fields"`
}

// getSettings devolve as configurações do canal ativo. Segredos nunca vão: só "configurado".
func (a *api) getSettings(w http.ResponseWriter, _ *http.Request) {
	d := a.settings
	byGroup := map[string][]fieldJSON{}
	for _, f := range d.Service.Fields(d.channel()) {
		byGroup[f.Group] = append(byGroup[f.Group], fieldJSON{
			Key: f.Key, Label: f.Label, Help: f.Help, Kind: string(f.Kind), Options: f.Options, Min: f.Min, Max: f.Max,
			Live: f.Live, Value: f.Value, IsSet: f.IsSet, Source: string(f.Source), PendingRestart: f.PendingRestart, Default: f.Default, Sensitive: f.Sensitive(),
		})
	}
	groups := []groupJSON{}
	for _, g := range settings.Groups {
		if fields := byGroup[g.ID]; len(fields) > 0 {
			groups = append(groups, groupJSON{ID: g.ID, Title: g.Title, Help: g.Help, Fields: fields})
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"channel": d.channel(), "encryption": d.Service.EncryptionEnabled(), "restartPending": d.Service.NeedsRestart(d.channel()), "groups": groups,
		"secondFactor": a.factor.Active(),
	})
}

// putSettings salva um lote (tudo ou nada). Ao mudar os nomes, avisa quantos lançamentos antigos parecem transferências entre
// contas suas: quem decide cancelá-los é a pessoa (own-transfers/apply).
func (a *api) putSettings(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Values   map[string]string `json:"values"`
		Password string            `json:"password"`
		Code     string            `json:"code"`
	}
	if !decodeJSON(w, r, maxSettingsBody, &body) {
		return
	}
	if len(body.Values) == 0 {
		writeError(w, http.StatusBadRequest, "corpo inválido")
		return
	}
	for k, v := range body.Values {
		if !utf8.ValidString(k) || !utf8.ValidString(v) {
			writeError(w, http.StatusBadRequest, "texto inválido")
			return
		}
	}

	d := a.settings
	for k := range body.Values {
		if def, ok := settings.Lookup(k); ok && def.Sensitive() {
			if !a.confirmSensitive(w, r, body.Password, body.Code) {
				return
			}
			break
		}
	}
	changed, err := d.Service.Set(r.Context(), body.Values)
	var invalid *settings.ValidationError
	switch {
	case errors.As(err, &invalid), errors.Is(err, settings.ErrNoEncryptionKey), errors.Is(err, settings.ErrUnknownKey):
		writeError(w, http.StatusBadRequest, err.Error())
		return
	case err != nil:
		a.fail(w, "salvar configurações", err)
		return
	}
	// Nunca registramos os valores: podem ser segredos.
	a.logger.Info("configurações salvas pelo dashboard", "keys", changed)
	a.record(r, "set", changed)

	resp := map[string]any{"changed": changed, "restartPending": d.Service.NeedsRestart(d.channel())}
	if _, ok := body.Values["OWN_NAMES"]; ok && d.Cleaner != nil {
		found, _, err := a.findOwnTransfers(r.Context())
		if err != nil {
			a.logger.Error("erro ao procurar transferências próprias", "error", err)
		} else if found.Count > 0 {
			resp["ownTransfers"] = found
		}
	}
	writeJSON(w, http.StatusOK, resp)
}

func (a *api) resetSetting(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Password string `json:"password"`
		Code     string `json:"code"`
	}
	if !decodeJSON(w, r, maxSettingsBody, &body) {
		return
	}
	key := r.PathValue("key")
	def, known := settings.Lookup(key)
	if !known {
		writeError(w, http.StatusNotFound, "configuração desconhecida")
		return
	}
	if def.Sensitive() && !a.confirmSensitive(w, r, body.Password, body.Code) {
		return
	}
	if err := a.settings.Service.Reset(r.Context(), key); err != nil {
		if errors.Is(err, settings.ErrUnknownKey) {
			writeError(w, http.StatusNotFound, "configuração desconhecida")
			return
		}
		a.fail(w, "restaurar configuração", err)
		return
	}
	a.record(r, "reset", []string{key})
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "restartPending": a.settings.Service.NeedsRestart(a.settings.channel())})
}

func (a *api) registerSettings(rt routes, deps *SettingsDeps) {
	a.coach.paidFn = func() bool { return deps.Service.Bool("GEMINI_PAID_PLAN") }
	a.confirmLimiter = newIPRateLimiter(8, time.Minute)
	writeLimiter := newIPRateLimiter(30, time.Minute)
	rt.mux.Handle("GET /api/settings", rt.protected(a.getSettings))
	rt.mux.Handle("PUT /api/settings", writeLimiter.middleware(rt.protected(jsonOnly(a.putSettings))))
	rt.mux.Handle("POST /api/settings/reset/{key}", writeLimiter.middleware(rt.protected(jsonOnly(a.resetSetting))))
	rt.mux.Handle("POST /api/settings/confirm", writeLimiter.middleware(rt.protected(jsonOnly(a.requestConfirmCode))))
	rt.mux.Handle("GET /api/settings/audit", rt.protected(a.auditLog))
	rt.mux.Handle("POST /api/settings/own-transfers/apply", writeLimiter.middleware(rt.protected(jsonOnly(a.applyOwnTransfers))))
	rt.mux.Handle("POST /api/settings/test/{target}", newIPRateLimiter(12, time.Minute).middleware(rt.protected(jsonOnly(a.testConnection))))
	rt.mux.Handle("POST /api/restart", newIPRateLimiter(3, time.Minute).middleware(rt.protected(jsonOnly(a.restart))))
}
