package httpserver

import (
	"context"
	"errors"
	"net/http"
	"regexp"
	"strings"
	"testing"
)

// chatBox faz as vezes do canal: guarda o que seria enviado e pode falhar.
type chatBox struct {
	sent []string
	err  error
}

func (c *chatBox) send(_ context.Context, text string) error {
	if c.err != nil {
		return c.err
	}
	c.sent = append(c.sent, text)
	return nil
}

var codeRe = regexp.MustCompile(`\d{8}`)

func (c *chatBox) lastCode(t *testing.T) string {
	t.Helper()
	if len(c.sent) == 0 {
		t.Fatal("nenhum código enviado")
	}
	return codeRe.FindString(c.sent[len(c.sent)-1])
}

func newFactorAPI(t *testing.T) (*settingsEnv, *Server, *chatBox) {
	t.Helper()
	box := &chatBox{}
	e, s := newSettingsAPIWith(t, goodKey, &SecondFactor{Enabled: true, Send: box.send})
	return e, s, box
}

func cookieNamed(rec interface{ Result() *http.Response }, name string) *http.Cookie {
	for _, c := range rec.Result().Cookies() {
		if c.Name == name {
			return c
		}
	}
	return nil
}

// loginWithCode faz os dois passos e devolve o cookie de sessão.
func loginWithCode(t *testing.T, s *Server, box *chatBox) *http.Cookie {
	t.Helper()
	rec := do(s, "POST", "/api/login", `{"password":"`+testPassword+`"}`, jsonHdr)
	ch := cookieNamed(rec, challengeCookie)
	if rec.Code != 200 || ch == nil || cookieNamed(rec, sessionCookie) != nil {
		t.Fatalf("senha certa = %d %s; desafio %v, sem sessão ainda", rec.Code, rec.Body, ch)
	}
	rec = do(s, "POST", "/api/login/code", `{"code":"`+box.lastCode(t)+`"}`, jsonHdr, ch)
	sess := cookieNamed(rec, sessionCookie)
	if rec.Code != 200 || sess == nil {
		t.Fatalf("código certo = %d %s", rec.Code, rec.Body)
	}
	return sess
}

func TestSecondFactor_LoginNeedsCode(t *testing.T) {
	_, s, box := newFactorAPI(t)
	if rec := do(s, "POST", "/api/login", `{"password":"errada"}`, jsonHdr); rec.Code != 401 || len(box.sent) != 0 {
		t.Fatalf("senha errada = %d, mensagens %v", rec.Code, box.sent)
	}
	c := loginWithCode(t, s, box)
	if !strings.Contains(box.sent[0], "Código de acesso") {
		t.Errorf("mensagem do código: %q", box.sent[0])
	}
	for _, path := range []string{"/api/me", "/api/settings"} {
		if rec := do(s, "GET", path, "", nil, c); rec.Code != 200 || !strings.Contains(rec.Body.String(), `"secondFactor":true`) {
			t.Fatalf("%s = %d %s", path, rec.Code, rec.Body)
		}
	}
}

func TestSecondFactor_CodeIsSingleUseAndBoundToBrowser(t *testing.T) {
	_, s, box := newFactorAPI(t)
	rec := do(s, "POST", "/api/login", `{"password":"`+testPassword+`"}`, jsonHdr)
	ch := cookieNamed(rec, challengeCookie)
	code := box.lastCode(t)

	other := &http.Cookie{Name: challengeCookie, Value: "outro-navegador"}
	if rec := do(s, "POST", "/api/login/code", `{"code":"`+code+`"}`, jsonHdr, other); rec.Code != 401 {
		t.Fatalf("código em outro navegador = %d", rec.Code)
	}
	if rec := do(s, "POST", "/api/login/code", `{"code":"`+code+`"}`, jsonHdr); rec.Code != 401 {
		t.Fatalf("código sem desafio = %d", rec.Code)
	}
	if rec := do(s, "POST", "/api/login/code", `{"code":"`+code+`"}`, jsonHdr, ch); rec.Code != 200 {
		t.Fatalf("código certo = %d %s", rec.Code, rec.Body)
	}
	if rec := do(s, "POST", "/api/login/code", `{"code":"`+code+`"}`, jsonHdr, ch); rec.Code != 401 {
		t.Fatalf("mesmo código de novo = %d", rec.Code)
	}
}

func TestSecondFactor_WrongCodesKillChallengeAndNotify(t *testing.T) {
	e, s, box := newFactorAPI(t)
	rec := do(s, "POST", "/api/login", `{"password":"`+testPassword+`"}`, jsonHdr)
	ch := cookieNamed(rec, challengeCookie)
	code := box.lastCode(t)
	wrong := "00000000"
	if code == wrong {
		wrong = "11111111"
	}
	for range 3 {
		do(s, "POST", "/api/login/code", `{"code":"`+wrong+`"}`, jsonHdr, ch)
	}
	if rec := do(s, "POST", "/api/login/code", `{"code":"`+code+`"}`, jsonHdr, ch); rec.Code != 401 {
		t.Fatalf("código certo depois de 3 erros = %d", rec.Code)
	}
	if len(e.notices) != 1 || !strings.Contains(e.notices[0], "códigos errados") {
		t.Fatalf("aviso de códigos errados: %v", e.notices)
	}
}

func TestSecondFactor_SendFailureLetsNobodyIn(t *testing.T) {
	_, s, box := newFactorAPI(t)
	box.err = errors.New("telegram fora")
	rec := do(s, "POST", "/api/login", `{"password":"`+testPassword+`"}`, jsonHdr)
	if rec.Code != 503 || cookieNamed(rec, sessionCookie) != nil || cookieNamed(rec, challengeCookie) != nil {
		t.Fatalf("canal fora = %d %s", rec.Code, rec.Body)
	}
}

func TestSecondFactor_ResendWaits(t *testing.T) {
	_, s, _ := newFactorAPI(t)
	do(s, "POST", "/api/login", `{"password":"`+testPassword+`"}`, jsonHdr)
	if rec := do(s, "POST", "/api/login", `{"password":"`+testPassword+`"}`, jsonHdr); rec.Code != 429 {
		t.Fatalf("reenvio imediato = %d", rec.Code)
	}
}

func TestSecondFactor_InactiveWithoutChannel(t *testing.T) {
	_, s := newSettingsAPIWith(t, goodKey, &SecondFactor{Enabled: true}) // sem Send: o canal não subiu
	c := login(t, s)
	if rec := do(s, "GET", "/api/me", "", nil, c); !strings.Contains(rec.Body.String(), `"secondFactor":false`) {
		t.Fatalf("/api/me = %s", rec.Body)
	}
}

func TestSecondFactor_DisabledByEnv(t *testing.T) {
	box := &chatBox{}
	_, s := newSettingsAPIWith(t, goodKey, &SecondFactor{Enabled: false, Send: box.send})
	login(t, s)
	if len(box.sent) != 0 {
		t.Fatalf("DASHBOARD_2FA=off não manda código: %v", box.sent)
	}
}

func TestSecondFactor_SensitiveSettingNeedsFreshCode(t *testing.T) {
	e, s, box := newFactorAPI(t)
	c := loginWithCode(t, s, box)
	put := func(extra string) int {
		return do(s, "PUT", "/api/settings", `{"values":{"PLUGGY_CLIENT_SECRET":"novo"}`+extra+`}`, jsonHdr, c).Code
	}
	if code := put(`,"password":"` + testPassword + `"`); code != 403 {
		t.Fatalf("com 2FA a senha não basta = %d", code)
	}
	if rec := do(s, "POST", "/api/settings/confirm", `{}`, jsonHdr, c); rec.Code != 200 {
		t.Fatalf("pedir código = %d %s", rec.Code, rec.Body)
	}
	code := box.lastCode(t)
	if !strings.Contains(box.sent[len(box.sent)-1], "confirmar a alteração") {
		t.Errorf("mensagem de confirmação: %q", box.sent[len(box.sent)-1])
	}
	if got := put(`,"code":"` + code + `"`); got != 200 || e.svc.Get("PLUGGY_CLIENT_SECRET") != "novo" {
		t.Fatalf("com o código = %d", got)
	}
	if got := put(`,"code":"` + code + `"`); got != 403 {
		t.Fatalf("o código vale uma alteração só = %d", got)
	}
	// O código de login não serve para confirmar, e vice-versa: cada um tem o seu desafio.
	if rec := do(s, "POST", "/api/settings/reset/PLUGGY_CLIENT_SECRET", `{"code":"`+code+`"}`, jsonHdr, c); rec.Code != 403 {
		t.Fatalf("reset com código usado = %d", rec.Code)
	}
}

func TestSecondFactor_ConfirmWithoutFactor(t *testing.T) {
	_, s := newSettingsAPI(t, goodKey)
	c := login(t, s)
	if rec := do(s, "POST", "/api/settings/confirm", `{}`, jsonHdr, c); rec.Code != 409 {
		t.Fatalf("sem 2FA, confirmar é com a senha = %d", rec.Code)
	}
}
