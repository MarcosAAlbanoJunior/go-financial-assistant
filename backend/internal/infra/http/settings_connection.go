package httpserver

import (
	"context"
	"net/http"
	"strconv"
	"time"
)

// testConnection confere as credenciais salvas. A resposta nunca repete o erro bruto do serviço externo.
func (a *api) testConnection(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), testTimeout)
	defer cancel()

	var ok bool
	var msg string
	switch r.PathValue("target") {
	case "pluggy":
		ok, msg = a.testPluggy(ctx)
	case "gemini":
		ok, msg = a.testGemini(ctx)
	case "telegram":
		ok, msg = a.testTelegram(ctx)
	default:
		writeError(w, http.StatusNotFound, "teste desconhecido")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": ok, "message": msg})
}

func (a *api) testPluggy(ctx context.Context) (bool, string) {
	s := a.settings.Service.Sync()
	if s.ClientID == "" || s.ClientSecret == "" {
		return false, "Preencha o Client ID e o Client Secret e salve."
	}
	checks, err := a.settings.PluggyCheck(ctx, s.ClientID, s.ClientSecret, s.ItemIDs)
	if err != nil {
		return false, "O Pluggy recusou as credenciais (Client ID ou Secret errados)."
	}
	bad := 0
	for _, c := range checks {
		if !c.OK {
			bad++
		}
	}
	switch {
	case len(checks) == 0:
		return true, "Credenciais aceitas. Falta informar os Item IDs dos bancos."
	case bad > 0:
		return false, "Credenciais aceitas, mas " + strconv.Itoa(bad) + " Item ID(s) não foram encontrados ou não estão autorizados no Meu Pluggy."
	}
	return true, "Tudo certo: credenciais aceitas e " + strconv.Itoa(len(checks)) + " conexão(ões) encontrada(s)."
}

func (a *api) testGemini(ctx context.Context) (bool, string) {
	key := a.settings.Service.Get("GEMINI_API_KEY")
	if key == "" {
		return false, "Preencha a chave do Gemini e salve."
	}
	if err := a.settings.GeminiPing(ctx, key); err != nil {
		return false, "O Gemini recusou a chave ou não respondeu. Confira a chave e a cota."
	}
	return true, "Chave aceita pelo Gemini."
}

func (a *api) testTelegram(ctx context.Context) (bool, string) {
	token := a.settings.Service.Get("TELEGRAM_BOT_TOKEN")
	if a.settings.channel() != "telegram" || token == "" {
		return false, "Preencha o token do bot e salve."
	}
	name, err := a.settings.TelegramPing(ctx, token)
	if err != nil {
		return false, "O Telegram recusou o token."
	}
	return true, "Token válido: bot @" + name + "."
}

// restart pede ao processo que encerre; o Docker o sobe de novo já com as configurações novas.
func (a *api) restart(w http.ResponseWriter, r *http.Request) {
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
