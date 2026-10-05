package httpserver

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	sessionCookie = "fa_session"
	sessionTTL    = 7 * 24 * time.Hour
)

// sessions emite e valida o cookie de sessão do dashboard. O cookie é "<expira>.<hmac>",
// sem estado no servidor. A chave é aleatória a cada boot: reiniciar o app encerra as sessões.
type sessions struct {
	key      []byte
	password [sha256.Size]byte
}

func newSessions(password string) (*sessions, error) {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return nil, fmt.Errorf("erro ao gerar chave de sessão: %w", err)
	}
	return &sessions{key: key, password: sha256.Sum256([]byte(password))}, nil
}

// checkPassword compara em tempo constante (via hash, para não vazar nem o tamanho da senha).
func (s *sessions) checkPassword(candidate string) bool {
	h := sha256.Sum256([]byte(candidate))
	return subtle.ConstantTimeCompare(h[:], s.password[:]) == 1
}

func (s *sessions) sign(expiry string) string {
	mac := hmac.New(sha256.New, s.key)
	mac.Write([]byte(expiry))
	return hex.EncodeToString(mac.Sum(nil))
}

func (s *sessions) issue(w http.ResponseWriter, r *http.Request) {
	expiry := strconv.FormatInt(time.Now().Add(sessionTTL).Unix(), 10)
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    expiry + "." + s.sign(expiry),
		Path:     "/",
		MaxAge:   int(sessionTTL.Seconds()),
		HttpOnly: true,
		Secure:   isHTTPS(r),
		SameSite: http.SameSiteStrictMode,
	})
}

func (s *sessions) clear(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name: sessionCookie, Path: "/", MaxAge: -1,
		HttpOnly: true, Secure: isHTTPS(r), SameSite: http.SameSiteStrictMode,
	})
}

func (s *sessions) valid(r *http.Request) bool {
	c, err := r.Cookie(sessionCookie)
	if err != nil {
		return false
	}
	expiry, sig, ok := strings.Cut(c.Value, ".")
	if !ok || !hmac.Equal([]byte(sig), []byte(s.sign(expiry))) {
		return false
	}
	exp, err := strconv.ParseInt(expiry, 10, 64)
	return err == nil && time.Now().Unix() < exp
}

// id identifica a sessão da requisição (o próprio cookie, já validado por require), para prender a ela o código de confirmação.
func (s *sessions) id(r *http.Request) string {
	c, err := r.Cookie(sessionCookie)
	if err != nil {
		return ""
	}
	return c.Value
}

// require responde 401 a quem não tem sessão válida.
func (s *sessions) require(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !s.valid(r) {
			writeError(w, http.StatusUnauthorized, "não autenticado")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// behindProxy diz se a requisição veio de um proxy da rede privada (o nginx do compose),
// único caso em que X-Real-IP e X-Forwarded-Proto são confiáveis.
func behindProxy(r *http.Request) bool {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return false
	}
	ip := net.ParseIP(host)
	return ip != nil && (ip.IsPrivate() || ip.IsLoopback())
}

// clientIP é o IP de quem fez a requisição, para o rate limit. Atrás do proxy, o nginx
// sobrescreve X-Real-IP com o IP real; fora dele o cabeçalho é ignorado, pois seria forjável.
func clientIP(r *http.Request) string {
	if behindProxy(r) {
		if ip := net.ParseIP(r.Header.Get("X-Real-IP")); ip != nil {
			return ip.String()
		}
	}
	ip, _, _ := net.SplitHostPort(r.RemoteAddr)
	return ip
}

func isHTTPS(r *http.Request) bool {
	return r.TLS != nil || (behindProxy(r) && r.Header.Get("X-Forwarded-Proto") == "https")
}

// sameOrigin bloqueia requisições de escrita vindas de outro site: se o navegador
// enviou Origin, o host precisa ser o mesmo da requisição.
func sameOrigin(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return true
	}
	u, err := url.Parse(origin)
	return err == nil && u.Host == r.Host
}
