package httpserver

import (
	"context"
	"encoding/json"
	"errors"
	"mime"
	"net/http"
	"strconv"
	"time"
	"unicode/utf8"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain"
	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain/ports"
	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/infra/pluggy"
	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/settings"
	"github.com/google/uuid"
)

const (
	maxSettingsBody = 16 << 10
	testTimeout     = 20 * time.Second
	maxExamples     = 5
)

// SettingsDeps é o que a página de configurações precisa. Os testes de conexão recebem as credenciais por parâmetro
// (as que estão salvas), para ficarem fora do pacote HTTP.
type SettingsDeps struct {
	Service      *settings.Service
	Channel      string
	Cleaner      ports.TransferCleaner
	PluggyCheck  func(ctx context.Context, clientID, clientSecret string, itemIDs []string) ([]pluggy.ItemCheck, error)
	TelegramPing func(ctx context.Context, token string) (string, error)
	GeminiPing   func(ctx context.Context, apiKey string) error
	Restart      func() // pede ao processo que encerre; o Docker (restart: unless-stopped) o sobe de novo
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
	for _, f := range d.Service.Fields(d.Channel) {
		byGroup[f.Group] = append(byGroup[f.Group], fieldJSON{
			Key: f.Key, Label: f.Label, Help: f.Help, Kind: string(f.Kind), Options: f.Options, Min: f.Min, Max: f.Max,
			Live: f.Live, Value: f.Value, IsSet: f.IsSet, Source: string(f.Source), PendingRestart: f.PendingRestart, Default: f.Default,
		})
	}
	groups := []groupJSON{}
	for _, g := range settings.Groups {
		if fields := byGroup[g.ID]; len(fields) > 0 {
			groups = append(groups, groupJSON{ID: g.ID, Title: g.Title, Help: g.Help, Fields: fields})
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"channel": d.Channel, "encryption": d.Service.EncryptionEnabled(), "restartPending": d.Service.NeedsRestart(d.Channel), "groups": groups,
	})
}

func (a *api) jsonWrite(w http.ResponseWriter, r *http.Request) bool {
	if mt, _, _ := mime.ParseMediaType(r.Header.Get("Content-Type")); mt != "application/json" || !sameOrigin(r) {
		writeError(w, http.StatusForbidden, "requisição não permitida")
		return false
	}
	return true
}

type transferExample struct {
	Description string  `json:"description"`
	Kind        string  `json:"kind"`
	Amount      float64 `json:"amount"`
}

type ownTransfers struct {
	Count    int               `json:"count"`
	Expense  float64           `json:"expense"`
	Income   float64           `json:"income"`
	Examples []transferExample `json:"examples"`
}

// findOwnTransfers procura, entre o que já foi gravado, o que é Pix/TED/DOC com os nomes atuais.
func (a *api) findOwnTransfers(ctx context.Context) (ownTransfers, []uuid.UUID, error) {
	var out ownTransfers
	matcher := domain.NewOwnTransferMatcher(a.settings.Service.List("OWN_NAMES"))
	cands, err := a.settings.Cleaner.OwnTransferCandidates(ctx)
	if err != nil {
		return out, nil, err
	}
	var ids []uuid.UUID
	for _, c := range cands {
		if !matcher.Match(c.Description) {
			continue
		}
		ids = append(ids, c.PaymentID)
		out.Count++
		if c.Kind == "EXPENSE" {
			out.Expense += c.Amount
		} else {
			out.Income += c.Amount
		}
		if len(out.Examples) < maxExamples {
			out.Examples = append(out.Examples, transferExample{Description: c.Description, Kind: c.Kind, Amount: c.Amount})
		}
	}
	return out, ids, nil
}

// putSettings salva um lote (tudo ou nada). Ao mudar os nomes, avisa quantos lançamentos antigos parecem transferências entre
// contas suas: quem decide cancelá-los é a pessoa (own-transfers/apply).
func (a *api) putSettings(w http.ResponseWriter, r *http.Request) {
	if !a.jsonWrite(w, r) {
		return
	}
	var body struct {
		Values map[string]string `json:"values"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxSettingsBody)).Decode(&body); err != nil || len(body.Values) == 0 {
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

	resp := map[string]any{"changed": changed, "restartPending": d.Service.NeedsRestart(d.Channel)}
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
	if !sameOrigin(r) {
		writeError(w, http.StatusForbidden, "requisição não permitida")
		return
	}
	key := r.PathValue("key")
	if err := a.settings.Service.Reset(r.Context(), key); err != nil {
		if errors.Is(err, settings.ErrUnknownKey) {
			writeError(w, http.StatusNotFound, "configuração desconhecida")
			return
		}
		a.fail(w, "restaurar configuração", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "restartPending": a.settings.Service.NeedsRestart(a.settings.Channel)})
}

// applyOwnTransfers cancela as transferências entre contas suas já gravadas (calculadas de novo no servidor, não vindas da tela).
func (a *api) applyOwnTransfers(w http.ResponseWriter, r *http.Request) {
	if !a.jsonWrite(w, r) {
		return
	}
	_, ids, err := a.findOwnTransfers(r.Context())
	if err != nil {
		a.fail(w, "procurar transferências próprias", err)
		return
	}
	var n int64
	if len(ids) > 0 {
		if n, err = a.settings.Cleaner.CancelPayments(r.Context(), ids); err != nil {
			a.fail(w, "cancelar transferências próprias", err)
			return
		}
	}
	a.logger.Info("transferências entre contas próprias canceladas", "count", n)
	writeJSON(w, http.StatusOK, map[string]any{"cancelled": n})
}

// testConnection confere as credenciais salvas. A resposta nunca repete o erro bruto do serviço externo.
func (a *api) testConnection(w http.ResponseWriter, r *http.Request) {
	if !a.jsonWrite(w, r) {
		return
	}
	d := a.settings
	ctx, cancel := context.WithTimeout(r.Context(), testTimeout)
	defer cancel()
	svc := d.Service

	result := func(ok bool, msg string) { writeJSON(w, http.StatusOK, map[string]any{"ok": ok, "message": msg}) }
	switch r.PathValue("target") {
	case "pluggy":
		s := svc.Sync()
		if s.ClientID == "" || s.ClientSecret == "" {
			result(false, "Preencha o Client ID e o Client Secret e salve.")
			return
		}
		checks, err := d.PluggyCheck(ctx, s.ClientID, s.ClientSecret, s.ItemIDs)
		if err != nil {
			result(false, "O Pluggy recusou as credenciais (Client ID ou Secret errados).")
			return
		}
		bad := 0
		for _, c := range checks {
			if !c.OK {
				bad++
			}
		}
		switch {
		case len(checks) == 0:
			result(true, "Credenciais aceitas. Falta informar os Item IDs dos bancos.")
		case bad > 0:
			result(false, "Credenciais aceitas, mas "+strconv.Itoa(bad)+" Item ID(s) não foram encontrados ou não estão autorizados no Meu Pluggy.")
		default:
			result(true, "Tudo certo: credenciais aceitas e "+strconv.Itoa(len(checks))+" conexão(ões) encontrada(s).")
		}
	case "gemini":
		key := svc.Get("GEMINI_API_KEY")
		if key == "" {
			result(false, "Preencha a chave do Gemini e salve.")
			return
		}
		if err := d.GeminiPing(ctx, key); err != nil {
			result(false, "O Gemini recusou a chave ou não respondeu. Confira a chave e a cota.")
			return
		}
		result(true, "Chave aceita pelo Gemini.")
	case "telegram":
		token := svc.Get("TELEGRAM_BOT_TOKEN")
		if d.Channel != "telegram" || token == "" {
			result(false, "Preencha o token do bot e salve.")
			return
		}
		name, err := d.TelegramPing(ctx, token)
		if err != nil {
			result(false, "O Telegram recusou o token.")
			return
		}
		result(true, "Token válido: bot @"+name+".")
	default:
		writeError(w, http.StatusNotFound, "teste desconhecido")
	}
}

// restart pede ao processo que encerre; o Docker o sobe de novo já com as configurações novas.
func (a *api) restart(w http.ResponseWriter, r *http.Request) {
	if !a.jsonWrite(w, r) {
		return
	}
	if a.settings.Restart == nil {
		writeError(w, http.StatusServiceUnavailable, "reinício indisponível")
		return
	}
	a.logger.Info("reinício pedido pelo dashboard")
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	go func() {
		time.Sleep(500 * time.Millisecond)
		a.settings.Restart()
	}()
}
