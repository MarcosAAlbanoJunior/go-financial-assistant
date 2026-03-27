package config

import (
	"testing"
)

func setEnv(t *testing.T, vars map[string]string) {
	t.Helper()
	for k, v := range vars {
		t.Setenv(k, v)
	}
}

func validEnv() map[string]string {
	return map[string]string{
		"PORT":                     "8080",
		"DATABASE_URL":             "postgres://user:pass@localhost/db",
		"GEMINI_API_KEY":    "gemini-key",
		"EVOLUTION_API_URL": "http://evolution:8080",
		"EVOLUTION_INSTANCE":       "my-instance",
		"EVOLUTION_API_KEY":        "evo-key",
		"OWNER_PHONE":              "5511999999999",
	}
}

func TestLoad_Success(t *testing.T) {
	setEnv(t, validEnv())

	cfg, err := Load()
	if err != nil {
		t.Fatalf("esperava sucesso, got: %v", err)
	}
	if cfg.Port != 8080 {
		t.Errorf("port esperada 8080, got %d", cfg.Port)
	}
	if cfg.DatabaseURL != "postgres://user:pass@localhost/db" {
		t.Errorf("DatabaseURL incorreta: %s", cfg.DatabaseURL)
	}
	if cfg.GeminiAPIKey != "gemini-key" {
		t.Errorf("GeminiAPIKey incorreta: %s", cfg.GeminiAPIKey)
	}
	if cfg.EvolutionAPIURL != "http://evolution:8080" {
		t.Errorf("EvolutionAPIURL incorreta: %s", cfg.EvolutionAPIURL)
	}
	if cfg.EvolutionInstance != "my-instance" {
		t.Errorf("EvolutionInstance incorreta: %s", cfg.EvolutionInstance)
	}
	if cfg.EvolutionAPIKey != "evo-key" {
		t.Errorf("EvolutionAPIKey incorreta: %s", cfg.EvolutionAPIKey)
	}
	if cfg.OwnerPhone != "5511999999999" {
		t.Errorf("OwnerPhone incorreto: %s", cfg.OwnerPhone)
	}
}

func TestLoad_DefaultPort(t *testing.T) {
	env := validEnv()
	delete(env, "PORT")
	setEnv(t, env)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("esperava sucesso, got: %v", err)
	}
	if cfg.Port != 8080 {
		t.Errorf("port default esperada 8080, got %d", cfg.Port)
	}
}

func TestLoad_DefaultEvolutionAPIURL(t *testing.T) {
	env := validEnv()
	delete(env, "EVOLUTION_API_URL")
	setEnv(t, env)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("esperava sucesso, got: %v", err)
	}
	if cfg.EvolutionAPIURL != "http://evolution:8080" {
		t.Errorf("EvolutionAPIURL default incorreta: %s", cfg.EvolutionAPIURL)
	}
}

func TestLoad_InvalidPort(t *testing.T) {
	env := validEnv()
	env["PORT"] = "nao-e-numero"
	setEnv(t, env)

	_, err := Load()
	if err == nil {
		t.Fatal("esperava erro de PORT inválida")
	}
}

func TestLoad_MissingDatabaseURL(t *testing.T) {
	env := validEnv()
	delete(env, "DATABASE_URL")
	setEnv(t, env)

	_, err := Load()
	if err == nil {
		t.Fatal("esperava erro de DATABASE_URL obrigatória")
	}
}

func TestLoad_MissingGeminiAPIKey(t *testing.T) {
	env := validEnv()
	delete(env, "GEMINI_API_KEY")
	setEnv(t, env)

	_, err := Load()
	if err == nil {
		t.Fatal("esperava erro de GEMINI_API_KEY obrigatória")
	}
}

func TestLoad_MissingEvolutionInstance(t *testing.T) {
	env := validEnv()
	delete(env, "EVOLUTION_INSTANCE")
	setEnv(t, env)

	_, err := Load()
	if err == nil {
		t.Fatal("esperava erro de EVOLUTION_INSTANCE obrigatória")
	}
}

func TestLoad_MissingEvolutionAPIKey(t *testing.T) {
	env := validEnv()
	delete(env, "EVOLUTION_API_KEY")
	setEnv(t, env)

	_, err := Load()
	if err == nil {
		t.Fatal("esperava erro de EVOLUTION_API_KEY obrigatória")
	}
}

func TestLoad_MissingOwnerPhone(t *testing.T) {
	env := validEnv()
	delete(env, "OWNER_PHONE")
	setEnv(t, env)

	_, err := Load()
	if err == nil {
		t.Fatal("esperava erro de OWNER_PHONE obrigatória")
	}
}

func TestLoad_MultipleErrors(t *testing.T) {
	setEnv(t, map[string]string{
		"PORT": "invalido",
	})

	_, err := Load()
	if err == nil {
		t.Fatal("esperava múltiplos erros")
	}
}

func TestParseAllowedNumbers_Single(t *testing.T) {
	result := parseAllowedNumbers("5511999999999")
	if _, ok := result["5511999999999"]; !ok {
		t.Error("número não encontrado no mapa")
	}
	if len(result) != 1 {
		t.Errorf("esperava 1 entrada, got %d", len(result))
	}
}

func TestParseAllowedNumbers_Multiple(t *testing.T) {
	result := parseAllowedNumbers("111, 222, 333")
	for _, n := range []string{"111", "222", "333"} {
		if _, ok := result[n]; !ok {
			t.Errorf("número '%s' não encontrado", n)
		}
	}
	if len(result) != 3 {
		t.Errorf("esperava 3 entradas, got %d", len(result))
	}
}

func TestParseAllowedNumbers_Empty(t *testing.T) {
	result := parseAllowedNumbers("")
	if len(result) != 0 {
		t.Errorf("esperava mapa vazio, got %d entradas", len(result))
	}
}

func TestLoad_AllowedNumbers(t *testing.T) {
	env := validEnv()
	env["ALLOWED_NUMBERS"] = "5511111111111, 5522222222222"
	setEnv(t, env)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("esperava sucesso, got: %v", err)
	}
	if len(cfg.AllowedNumbers) != 2 {
		t.Errorf("esperava 2 números permitidos, got %d", len(cfg.AllowedNumbers))
	}
}
