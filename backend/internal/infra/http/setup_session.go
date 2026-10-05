package httpserver

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const (
	setupCookie = "fa_setup"
	setupTTL    = time.Hour
)

// setupSessions é a sessão do setup, separada da do dashboard: abre com o SETUP_TOKEN, só vale para /api/setup e dura
// uma hora, renovada a cada passo. O cookie é "<id>.<expira>.<hmac>": o id prende o código de confirmação do chat a
// este navegador. A chave é aleatória a cada boot (reiniciar pede o token de novo).
type setupSessions struct {
	key []byte
	now func() time.Time
}

func newSetupSessions() (*setupSessions, error) {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return nil, fmt.Errorf("erro ao gerar chave da sessão de setup: %w", err)
	}
	return &setupSessions{key: key, now: time.Now}, nil
}

func (s *setupSessions) sign(payload string) string {
	mac := hmac.New(sha256.New, s.key)
	mac.Write([]byte("setup:" + payload))
	return hex.EncodeToString(mac.Sum(nil))
}

func (s *setupSessions) set(w http.ResponseWriter, r *http.Request, id string) {
	payload := id + "." + strconv.FormatInt(s.now().Add(setupTTL).Unix(), 10)
	http.SetCookie(w, &http.Cookie{
		Name: setupCookie, Value: payload + "." + s.sign(payload), Path: "/api/setup", MaxAge: int(setupTTL.Seconds()),
		HttpOnly: true, Secure: isHTTPS(r), SameSite: http.SameSiteStrictMode,
	})
}

// open abre uma sessão nova (token certo).
func (s *setupSessions) open(w http.ResponseWriter, r *http.Request) error {
	id, err := randomToken()
	if err != nil {
		return err
	}
	s.set(w, r, id)
	return nil
}

// clear encerra a sessão. Tira antes a renovação que require já tinha posto na resposta: um cookie só, o que apaga.
func (s *setupSessions) clear(w http.ResponseWriter, r *http.Request) {
	kept := w.Header()["Set-Cookie"][:0]
	for _, c := range w.Header()["Set-Cookie"] {
		if !strings.HasPrefix(c, setupCookie+"=") {
			kept = append(kept, c)
		}
	}
	w.Header()["Set-Cookie"] = kept
	http.SetCookie(w, &http.Cookie{
		Name: setupCookie, Path: "/api/setup", MaxAge: -1, HttpOnly: true, Secure: isHTTPS(r), SameSite: http.SameSiteStrictMode,
	})
}

// id devolve a sessão válida da requisição ("" sem sessão).
func (s *setupSessions) id(r *http.Request) string {
	c, err := r.Cookie(setupCookie)
	if err != nil {
		return ""
	}
	payload, sig, ok := cutLast(c.Value)
	if !ok || !hmac.Equal([]byte(sig), []byte(s.sign(payload))) {
		return ""
	}
	id, expiry, ok := strings.Cut(payload, ".")
	exp, err := strconv.ParseInt(expiry, 10, 64)
	if !ok || id == "" || err != nil || s.now().Unix() >= exp {
		return ""
	}
	return id
}

// require responde 401 sem sessão de setup e renova a que existe (mais uma hora a cada passo).
func (s *setupSessions) require(next func(w http.ResponseWriter, r *http.Request, id string)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := s.id(r)
		if id == "" {
			writeError(w, http.StatusUnauthorized, "sessão de setup expirada: cole o token de novo")
			return
		}
		s.set(w, r, id)
		next(w, r, id)
	}
}

func cutLast(v string) (before, after string, ok bool) {
	i := strings.LastIndex(v, ".")
	if i < 0 {
		return "", "", false
	}
	return v[:i], v[i+1:], true
}
