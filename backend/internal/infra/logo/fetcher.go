// Package logo baixa o logo de um banco (imageUrl do conector do Pluggy) de forma segura: a URL
// vem de terceiros, então o app só a acessa com as defesas abaixo e depois serve a imagem da própria origem.
package logo

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"syscall"
	"time"
)

const (
	MaxBytes     = 256 << 10
	fetchTimeout = 10 * time.Second
	maxRedirect  = 3
)

var (
	ErrNotHTTPS      = errors.New("logo: só https é permitido")
	ErrBlockedHost   = errors.New("logo: endereço de rede interna bloqueado")
	ErrTooLarge      = errors.New("logo: arquivo maior que o limite")
	ErrUnsupported   = errors.New("logo: tipo de imagem não aceito")
	ErrBadStatus     = errors.New("logo: resposta inesperada do servidor")
	ErrTooManyRedirs = errors.New("logo: redirecionamentos demais")
)

type Fetcher struct {
	client *http.Client
}

// New devolve um Fetcher que recusa, na hora de conectar (depois do DNS, então vale também contra
// rebinding), qualquer IP privado, loopback, link-local ou não roteável, e não usa proxy.
func New() *Fetcher {
	dialer := &net.Dialer{Timeout: 5 * time.Second, Control: func(_, address string, _ syscall.RawConn) error {
		host, _, err := net.SplitHostPort(address)
		if err != nil || BlockedIP(net.ParseIP(host)) {
			return ErrBlockedHost
		}
		return nil
	}}
	return newFetcher(&http.Transport{
		Proxy:               nil,
		DialContext:         dialer.DialContext,
		TLSHandshakeTimeout: 5 * time.Second,
		DisableKeepAlives:   true,
	})
}

func newFetcher(rt http.RoundTripper) *Fetcher {
	return &Fetcher{client: &http.Client{Transport: rt, Timeout: fetchTimeout, CheckRedirect: checkRedirect}}
}

func checkRedirect(req *http.Request, via []*http.Request) error {
	if len(via) > maxRedirect {
		return ErrTooManyRedirs
	}
	if req.URL.Scheme != "https" {
		return ErrNotHTTPS
	}
	return nil
}

// BlockedIP diz se o IP não deve ser acessado: nil, loopback, privado, link-local, multicast,
// não especificado ou fora da internet pública (CGNAT, documentação, 6to4 etc.).
func BlockedIP(ip net.IP) bool {
	if ip == nil || ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() ||
		ip.IsMulticast() || ip.IsUnspecified() || ip.IsInterfaceLocalMulticast() {
		return true
	}
	for _, n := range reservedNets {
		if n.Contains(ip) {
			return true
		}
	}
	return false
}

var reservedNets = func() []*net.IPNet {
	var out []*net.IPNet
	for _, cidr := range []string{
		"0.0.0.0/8", "100.64.0.0/10", "192.0.0.0/24", "192.0.2.0/24", "198.18.0.0/15",
		"198.51.100.0/24", "203.0.113.0/24", "240.0.0.0/4", "64:ff9b::/96", "2001:db8::/32", "2002::/16",
	} {
		_, n, _ := net.ParseCIDR(cidr)
		out = append(out, n)
	}
	return out
}()

// Fetch implementa ports.LogoFetcher.
func (f *Fetcher) Fetch(ctx context.Context, imageURL string) ([]byte, string, error) {
	u, err := url.Parse(imageURL)
	if err != nil || u.Scheme != "https" || u.Hostname() == "" {
		return nil, "", ErrNotHTTPS
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, "", ErrNotHTTPS
	}
	req.Header.Set("Accept", "image/png,image/jpeg,image/webp,image/svg+xml")

	resp, err := f.client.Do(req)
	if err != nil {
		// O erro original traz a URL; reduzimos à causa.
		var urlErr *url.Error
		if errors.As(err, &urlErr) {
			err = urlErr.Err
		}
		return nil, "", fmt.Errorf("logo: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, "", ErrBadStatus
	}
	if resp.ContentLength > MaxBytes {
		return nil, "", ErrTooLarge
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, MaxBytes+1))
	if err != nil {
		return nil, "", fmt.Errorf("logo: erro ao ler imagem: %w", err)
	}
	if len(data) > MaxBytes {
		return nil, "", ErrTooLarge
	}
	mime, ok := detectMime(data)
	if !ok {
		return nil, "", ErrUnsupported
	}
	return data, mime, nil
}

// detectMime decide o tipo pelo conteúdo, nunca pelo cabeçalho do servidor.
func detectMime(data []byte) (string, bool) {
	switch m := http.DetectContentType(data); m {
	case "image/png", "image/jpeg", "image/webp":
		return m, true
	}
	head := strings.ToLower(strings.TrimSpace(string(data[:min(len(data), 512)])))
	if strings.HasPrefix(head, "<svg") || (strings.HasPrefix(head, "<?xml") && strings.Contains(head, "<svg")) {
		return "image/svg+xml", true
	}
	return "", false
}
