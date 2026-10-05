package auth

import (
	"strings"
	"testing"
)

func TestPassword_HashAndVerify(t *testing.T) {
	h, err := HashPassword("uma senha boa de 12+")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(h, "$argon2id$v=19$m=65536,t=3,p=2$") || strings.Contains(h, "uma senha") {
		t.Fatalf("formato do hash: %s", h)
	}
	if !VerifyPassword(h, "uma senha boa de 12+") {
		t.Fatal("senha certa recusada")
	}
	if VerifyPassword(h, "uma senha boa de 12") {
		t.Fatal("senha errada aceita")
	}
	h2, _ := HashPassword("uma senha boa de 12+")
	if h == h2 {
		t.Fatal("o sal deveria mudar a cada hash")
	}
}

func TestPassword_BadHashNeverAccepts(t *testing.T) {
	for _, h := range []string{"", "texto-puro", "$argon2i$v=19$m=1,t=1,p=1$c2Fs$aGFzaA", "$argon2id$v=19$m=1,t=0,p=1$c2Fs$aGFzaA", "$argon2id$v=19$m=8,t=1,p=1$c2Fs$"} {
		if VerifyPassword(h, "") || VerifyPassword(h, "texto-puro") {
			t.Fatalf("hash inválido aceitou: %q", h)
		}
	}
}
