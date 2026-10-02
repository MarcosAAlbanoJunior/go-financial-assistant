package httpserver

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/settings"
)

// confirmPassword exige a senha do dashboard de novo para mexer no que é sensível (segredos, bancos conectados, quem
// manda no bot). Uma sessão roubada não basta. Tentativas têm limite próprio e a senha errada avisa a pessoa no chat.
func (a *api) confirmPassword(w http.ResponseWriter, r *http.Request, password string) bool {
	ip := clientIP(r)
	if a.confirmLimiter.blocked(ip) {
		writeError(w, http.StatusTooManyRequests, "muitas tentativas; espere um minuto")
		return false
	}
	if a.sessions.checkPassword(password) {
		return true
	}
	a.confirmLimiter.record(ip)
	a.logger.Warn("confirmação de senha recusada nas configurações", "ip", clientIP(r))
	if n := time.Now().UnixNano(); n-a.lastFailNotice.Load() > int64(failedNoticeEvery) {
		a.lastFailNotice.Store(n)
		a.notify(r.Context(), "⚠️ Alguém errou a senha ao tentar alterar uma configuração sensível do dashboard ("+time.Now().Format("02/01 15:04")+"). Se não foi você, troque a senha do dashboard.")
	}
	writeError(w, http.StatusForbidden, "senha incorreta")
	return false
}

func (a *api) notify(ctx context.Context, text string) {
	if a.settings.Notify != nil {
		a.settings.Notify(ctx, text)
	}
}

// record guarda no histórico quem mudou o quê (sem o valor) e avisa no chat quando o que mudou é sensível.
func (a *api) record(r *http.Request, action string, keys []string) {
	var labels []string
	for _, k := range keys {
		def, _ := settings.Lookup(k)
		if a.settings.Audit != nil {
			if err := a.settings.Audit.RecordAudit(r.Context(), settings.AuditEntry{Action: action, Key: k, Sensitive: def.Sensitive(), IP: clientIP(r)}); err != nil {
				a.logger.Error("erro ao registrar alteração", "error", err)
			}
		}
		if def.Sensitive() {
			labels = append(labels, def.Label)
		}
	}
	if len(labels) > 0 {
		verb := "alterada"
		if action == "reset" {
			verb = "restaurada"
		}
		a.notify(r.Context(), "🔐 Configuração sensível "+verb+" pelo dashboard: "+strings.Join(labels, ", ")+" ("+time.Now().Format("02/01 15:04")+"). Se não foi você, troque a senha do dashboard e as chaves.")
	}
}

type auditJSON struct {
	At        time.Time `json:"at"`
	Action    string    `json:"action"`
	Key       string    `json:"key"`
	Label     string    `json:"label"`
	Sensitive bool      `json:"sensitive"`
	IP        string    `json:"ip"`
}

// auditLog lista as últimas alterações de configuração (sem valores).
func (a *api) auditLog(w http.ResponseWriter, r *http.Request) {
	out := []auditJSON{}
	if a.settings.Audit != nil {
		entries, err := a.settings.Audit.RecentAudit(r.Context(), 20)
		if err != nil {
			a.fail(w, "histórico de configurações", err)
			return
		}
		for _, e := range entries {
			def, _ := settings.Lookup(e.Key)
			label := def.Label
			if label == "" {
				label = e.Key
			}
			out = append(out, auditJSON{At: e.At, Action: e.Action, Key: e.Key, Label: label, Sensitive: e.Sensitive, IP: e.IP})
		}
	}
	writeJSON(w, http.StatusOK, out)
}

const failedNoticeEvery = 10 * time.Minute // no máximo um aviso de senha errada por janela, para não virar spam
