package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadSecretKey(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "key")
	os.WriteFile(file, []byte("  chave-no-arquivo-123456\n"), 0o600) //nolint:errcheck

	if got, err := ReadSecretKey("da-variavel-1234567", ""); err != nil || got != "da-variavel-1234567" {
		t.Errorf("sem arquivo usa a variável: %q %v", got, err)
	}
	if got, err := ReadSecretKey("da-variavel-1234567", file); err != nil || got != "chave-no-arquivo-123456" {
		t.Errorf("o arquivo vence a variável, sem espaços nem quebra de linha: %q %v", got, err)
	}
	if got, err := ReadSecretKey("", ""); err != nil || got != "" {
		t.Errorf("sem nada = segredos desligados: %q %v", got, err)
	}
	if _, err := ReadSecretKey("", filepath.Join(dir, "nao-existe")); err == nil {
		t.Error("arquivo configurado e ausente deve dar erro (nunca cair em silêncio para 'sem chave')")
	}
	empty := filepath.Join(dir, "vazio")
	os.WriteFile(empty, []byte("\n"), 0o600) //nolint:errcheck
	if _, err := ReadSecretKey("", empty); err == nil {
		t.Error("arquivo vazio deve dar erro")
	}
}
