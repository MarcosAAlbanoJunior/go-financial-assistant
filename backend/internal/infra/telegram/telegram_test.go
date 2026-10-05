package telegram

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/usecase/ledger"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/chat"
)

const testToken = "123:SECRET"

func newTestClient(t *testing.T, h http.HandlerFunc) *Client {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	c := NewClient(testToken)
	c.baseURL = srv.URL
	return c
}

func reply(w http.ResponseWriter, result string) {
	io.WriteString(w, `{"ok":true,"result":`+result+`}`)
}

func TestToHTML(t *testing.T) {
	got := toHTML("*Extrato* de <b> & Cia", 100)
	want := "<b>Extrato</b> de &lt;b&gt; &amp; Cia"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
	if got := toHTML(strings.Repeat("a", 50), 10); got != strings.Repeat("a", 10)+"…" {
		t.Errorf("truncamento incorreto: %q", got)
	}
}

func TestSendText_Params(t *testing.T) {
	var body map[string]string
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/bot"+testToken+"/sendMessage" {
			t.Errorf("path inesperado: %s", r.URL.Path)
		}
		json.NewDecoder(r.Body).Decode(&body)
		reply(w, `{}`)
	})

	if _, err := c.SendText(context.Background(), "42", "*oi*"); err != nil {
		t.Fatal(err)
	}
	if body["chat_id"] != "42" || body["text"] != "<b>oi</b>" || body["parse_mode"] != "HTML" {
		t.Errorf("params inesperados: %v", body)
	}
}

func TestSendDocument_Multipart(t *testing.T) {
	var chatID, caption, filename string
	var content []byte
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			t.Fatal(err)
		}
		chatID, caption = r.FormValue("chat_id"), r.FormValue("caption")
		f, hdr, err := r.FormFile("document")
		if err != nil {
			t.Fatal(err)
		}
		filename = hdr.Filename
		content, _ = io.ReadAll(f)
		reply(w, `{}`)
	})

	if _, err := c.SendDocument(context.Background(), "42", "gastos.csv", []byte("a,b"), "*mar*"); err != nil {
		t.Fatal(err)
	}
	if chatID != "42" || caption != "<b>mar</b>" || filename != "gastos.csv" || string(content) != "a,b" {
		t.Errorf("upload incorreto: %q %q %q %q", chatID, caption, filename, content)
	}
}

func TestAPIError_IsReported(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `{"ok":false,"error_code":409,"description":"Conflict: webhook is active"}`)
	})
	_, err := c.getUpdates(context.Background(), 0, 0)
	if err == nil || !strings.Contains(err.Error(), "Conflict") {
		t.Errorf("esperava erro da API, got %v", err)
	}
}

func TestErrors_NeverLeakToken(t *testing.T) {
	c := NewClient(testToken)
	c.baseURL = "http://127.0.0.1:1" // porta fechada: erro de conexão carrega a URL
	_, err := c.GetMe(context.Background())
	if err == nil {
		t.Fatal("esperava erro de conexão")
	}
	if strings.Contains(err.Error(), "SECRET") {
		t.Errorf("token vazou no erro: %v", err)
	}
}

func TestDownload(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/bot" + testToken + "/getFile":
			reply(w, `{"file_path":"photos/a.jpg","file_size":3}`)
		case "/file/bot" + testToken + "/photos/a.jpg":
			io.WriteString(w, "abc")
		default:
			t.Errorf("path inesperado: %s", r.URL.Path)
		}
	})
	data, err := c.Download(context.Background(), "FILE")
	if err != nil || string(data) != "abc" {
		t.Errorf("got %q, %v", data, err)
	}
}

func TestDownload_TooLarge(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		reply(w, `{"file_path":"x","file_size":99999999}`)
	})
	if _, err := c.Download(context.Background(), "FILE"); err == nil {
		t.Error("esperava erro para arquivo > 20 MB")
	}
}

type fakeHandler struct{ got []chat.Message }

func (f *fakeHandler) Handle(_ context.Context, m chat.Message) (*ledger.ExpenseOutput, error) {
	f.got = append(f.got, m)
	return nil, errors.New("ignorado")
}

func newTestBot() (*Bot, *fakeHandler) {
	h := &fakeHandler{}
	return NewBot(NewClient(testToken), 42, h, slog.New(slog.NewTextHandler(io.Discard, nil))), h
}

func TestProcess_OnlyOwnerInPrivateChat(t *testing.T) {
	cases := []struct {
		name    string
		msg     *message
		handled bool
	}{
		{"dono", &message{Text: "oi", Chat: chatInfo{Type: "private"}, From: &user{ID: 42}}, true},
		{"outro usuário", &message{Text: "oi", Chat: chatInfo{Type: "private"}, From: &user{ID: 7}}, false},
		{"grupo", &message{Text: "oi", Chat: chatInfo{Type: "group"}, From: &user{ID: 42}}, false},
		{"sem remetente", &message{Text: "oi", Chat: chatInfo{Type: "private"}}, false},
		{"sem message", nil, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			b, h := newTestBot()
			b.process(context.Background(), update{UpdateID: 1, Message: tc.msg})
			if (len(h.got) == 1) != tc.handled {
				t.Errorf("handled=%v, esperado %v", len(h.got) == 1, tc.handled)
			}
		})
	}
}

func TestToChatMessage(t *testing.T) {
	b, _ := newTestBot()

	if m := b.toChatMessage(&message{Text: "oi"}); m.Text != "oi" || m.Image != nil || m.Document != nil {
		t.Errorf("texto: %+v", m)
	}

	m := b.toChatMessage(&message{Caption: "nota", Photo: []photoSize{{FileID: "small"}, {FileID: "big"}}})
	if m.Image == nil || m.Image.MimeType != "image/jpeg" || m.Image.Caption != "nota" {
		t.Errorf("foto: %+v", m)
	}

	if m := b.toChatMessage(&message{Document: &document{FileID: "d", MimeType: "image/png"}}); m.Image == nil || m.Document != nil {
		t.Errorf("imagem como arquivo deveria ser Image: %+v", m)
	}

	if m := b.toChatMessage(&message{Document: &document{FileID: "d", MimeType: "application/pdf"}}); m.Document == nil || m.Document.MimeType != "application/pdf" {
		t.Errorf("pdf: %+v", m)
	}
}

func TestRun_AdvancesOffsetAndStopsOnCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var offsets []float64
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		var params map[string]any
		json.NewDecoder(r.Body).Decode(&params)
		offsets = append(offsets, params["offset"].(float64))
		if len(offsets) == 1 {
			reply(w, `[{"update_id":10,"message":{"text":"oi","chat":{"type":"private"},"from":{"id":42}}}]`)
			return
		}
		cancel()
		reply(w, `[]`)
	})

	h := &fakeHandler{}
	bot := NewBot(c, 42, h, slog.New(slog.NewTextHandler(io.Discard, nil)))
	bot.Run(ctx)

	if len(h.got) != 1 || h.got[0].Text != "oi" {
		t.Errorf("mensagem não entregue: %+v", h.got)
	}
	if len(offsets) != 2 || offsets[0] != 0 || offsets[1] != 11 {
		t.Errorf("offset deveria avançar para update_id+1: %v", offsets)
	}
}

// O texto do /saldos usa *negrito* e traz nomes vindos do banco: o canal escapa o HTML e converte o negrito.
func TestToHTML_BalancesText(t *testing.T) {
	got := toHTML("💰 *Seus saldos*\n🏦 AT&T <b>x</b>  R$ 1,00", maxTextRunes)
	if want := "💰 <b>Seus saldos</b>\n🏦 AT&amp;T &lt;b&gt;x&lt;/b&gt;  R$ 1,00"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// Depois do setup o bot começa no offset seguinte ao /start: nada da espera é reprocessado.
func TestRun_StartsFromGivenOffset(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var first float64 = -1
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		var params map[string]any
		json.NewDecoder(r.Body).Decode(&params)
		if first < 0 {
			first = params["offset"].(float64)
		}
		cancel()
		reply(w, `[]`)
	})
	NewBot(c, 42, &fakeHandler{}, slog.New(slog.NewTextHandler(io.Discard, nil))).StartFrom(77).Run(ctx)
	if first != 77 {
		t.Fatalf("primeiro offset = %v, queria 77", first)
	}
}

func TestLatestOffset(t *testing.T) {
	var asked float64
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		var params map[string]any
		json.NewDecoder(r.Body).Decode(&params)
		asked = params["offset"].(float64)
		reply(w, `[{"update_id":41}]`)
	})
	got, err := c.LatestOffset(context.Background())
	if err != nil || got != 42 || asked != -1 {
		t.Fatalf("offset = %d %v (pediu %v)", got, err, asked)
	}
	empty := newTestClient(t, func(w http.ResponseWriter, r *http.Request) { reply(w, `[]`) })
	if got, err := empty.LatestOffset(context.Background()); err != nil || got != 0 {
		t.Fatalf("sem updates = %d %v", got, err)
	}
}

// Só a primeira conversa privada conta; mensagens de grupo passam e o offset avança até ela.
func TestFirstPrivate(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		reply(w, `[
			{"update_id":10,"message":{"text":"oi","chat":{"type":"group"},"from":{"id":7,"first_name":"Grupo"}}},
			{"update_id":11,"message":{"text":"/start","chat":{"type":"private"},"from":{"id":42,"first_name":"Maria","last_name":"Silva","username":"maria"}}},
			{"update_id":12,"message":{"text":"/start","chat":{"type":"private"},"from":{"id":43,"first_name":"Outro"}}}
		]`)
	})
	s, next, err := c.FirstPrivate(context.Background(), 10)
	if err != nil || s == nil || s.ID != 42 || s.Name != "Maria Silva" || s.Username != "maria" || next != 12 {
		t.Fatalf("got %+v next=%d err=%v", s, next, err)
	}

	groups := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		reply(w, `[{"update_id":20,"message":{"chat":{"type":"supergroup"},"from":{"id":7}}}]`)
	})
	if s, next, err := groups.FirstPrivate(context.Background(), 20); err != nil || s != nil || next != 21 {
		t.Fatalf("só grupo: %+v next=%d err=%v", s, next, err)
	}
}
