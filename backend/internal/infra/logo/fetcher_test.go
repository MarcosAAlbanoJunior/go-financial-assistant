package logo

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// png mínimo válido (assinatura + IHDR) o bastante para o sniffing.
var pngBytes = append([]byte("\x89PNG\r\n\x1a\n"), make([]byte, 32)...)

func tlsFetcher(srv *httptest.Server) *Fetcher {
	f := newFetcher(srv.Client().Transport)
	return f
}

func serve(t *testing.T, h http.HandlerFunc) *httptest.Server {
	t.Helper()
	srv := httptest.NewTLSServer(h)
	t.Cleanup(srv.Close)
	return srv
}

func TestFetch_AcceptsPNGByContentNotHeader(t *testing.T) {
	srv := serve(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html") // mentira: o conteúdo é que vale
		w.Write(pngBytes)
	})
	data, mime, err := tlsFetcher(srv).Fetch(context.Background(), srv.URL+"/a.png")
	if err != nil || mime != "image/png" || len(data) != len(pngBytes) {
		t.Fatalf("got %d bytes, %q, %v", len(data), mime, err)
	}
}

func TestFetch_AcceptsSVG(t *testing.T) {
	srv := serve(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte(`<?xml version="1.0"?><svg xmlns="http://www.w3.org/2000/svg"></svg>`))
	})
	_, mime, err := tlsFetcher(srv).Fetch(context.Background(), srv.URL)
	if err != nil || mime != "image/svg+xml" {
		t.Fatalf("got %q, %v", mime, err)
	}
}

func TestFetch_Rejects(t *testing.T) {
	cases := map[string]struct {
		handler http.HandlerFunc
		want    error
	}{
		"html": {func(w http.ResponseWriter, _ *http.Request) {
			w.Write([]byte("<html><script>alert(1)</script></html>"))
		}, ErrUnsupported},
		"grande": {func(w http.ResponseWriter, _ *http.Request) {
			w.Write(append(pngBytes, []byte(strings.Repeat("x", MaxBytes))...))
		}, ErrTooLarge},
		"status": {func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNotFound) }, ErrBadStatus},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			srv := serve(t, c.handler)
			if _, _, err := tlsFetcher(srv).Fetch(context.Background(), srv.URL); !errors.Is(err, c.want) {
				t.Fatalf("esperava %v, got %v", c.want, err)
			}
		})
	}
}

func TestFetch_OnlyHTTPS(t *testing.T) {
	for _, u := range []string{"http://example.com/a.png", "file:///etc/passwd", "ftp://x/y", "//example.com/a", "", "https://"} {
		if _, _, err := New().Fetch(context.Background(), u); !errors.Is(err, ErrNotHTTPS) {
			t.Errorf("%q: esperava ErrNotHTTPS, got %v", u, err)
		}
	}
}

// O Fetcher de produção não conecta em loopback (o httptest escuta em 127.0.0.1).
func TestFetch_BlocksInternalHost(t *testing.T) {
	srv := serve(t, func(w http.ResponseWriter, _ *http.Request) { w.Write(pngBytes) })
	if _, _, err := New().Fetch(context.Background(), srv.URL); !errors.Is(err, ErrBlockedHost) {
		t.Fatalf("esperava ErrBlockedHost, got %v", err)
	}
}

func TestCheckRedirect(t *testing.T) {
	req := func(u string) *http.Request { r, _ := http.NewRequest("GET", u, nil); return r }
	if err := checkRedirect(req("http://example.com"), nil); !errors.Is(err, ErrNotHTTPS) {
		t.Errorf("redirecionamento para http deveria ser recusado, got %v", err)
	}
	if err := checkRedirect(req("https://example.com"), make([]*http.Request, maxRedirect+1)); !errors.Is(err, ErrTooManyRedirs) {
		t.Errorf("redirecionamentos demais deveriam ser recusados, got %v", err)
	}
	if err := checkRedirect(req("https://example.com"), nil); err != nil {
		t.Errorf("https deveria passar, got %v", err)
	}
}

func TestBlockedIP(t *testing.T) {
	for ip, want := range map[string]bool{
		"127.0.0.1": true, "::1": true, "10.0.0.5": true, "192.168.1.1": true, "172.16.0.1": true,
		"169.254.169.254": true, "100.64.0.1": true, "0.0.0.0": true, "fe80::1": true, "fc00::1": true,
		"::ffff:127.0.0.1": true, "224.0.0.1": true,
		"8.8.8.8": false, "1.1.1.1": false, "2606:4700:4700::1111": false,
	} {
		if got := BlockedIP(net.ParseIP(ip)); got != want {
			t.Errorf("BlockedIP(%s) = %v, quer %v", ip, got, want)
		}
	}
	if !BlockedIP(nil) {
		t.Error("nil deveria ser bloqueado")
	}
}
