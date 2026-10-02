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
		"PORT":               "8080",
		"DATABASE_URL":       "postgres://user:pass@localhost/db",
		"GEMINI_API_KEY":     "gemini-key",
		"EVOLUTION_API_URL":  "http://evolution:8080",
		"EVOLUTION_INSTANCE": "my-instance",
		"EVOLUTION_API_KEY":  "evo-key",
		"OWNER_PHONE":        "5511999999999",
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
	if cfg.EvolutionAPIURL != "http://evolution:8082" {
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
	if _, ok := result["5511999999999@s.whatsapp.net"]; !ok {
		t.Error("número não encontrado no mapa com sufixo @s.whatsapp.net")
	}
	if len(result) != 1 {
		t.Errorf("esperava 1 entrada, got %d", len(result))
	}
}

func TestParseAllowedNumbers_Multiple(t *testing.T) {
	result := parseAllowedNumbers("111, 222, 333")
	for _, n := range []string{"111@s.whatsapp.net", "222@s.whatsapp.net", "333@s.whatsapp.net"} {
		if _, ok := result[n]; !ok {
			t.Errorf("número '%s' não encontrado", n)
		}
	}
	if len(result) != 3 {
		t.Errorf("esperava 3 entradas, got %d", len(result))
	}
}

func TestParseAllowedNumbers_AlreadyHasSuffix(t *testing.T) {
	result := parseAllowedNumbers("5511999999999@s.whatsapp.net")
	if _, ok := result["5511999999999@s.whatsapp.net"]; !ok {
		t.Error("número com sufixo existente não deve ser duplicado")
	}
	if len(result) != 1 {
		t.Errorf("esperava 1 entrada, got %d", len(result))
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

func telegramEnv() map[string]string {
	return map[string]string{
		"PORT":               "8080",
		"DATABASE_URL":       "postgres://user:pass@localhost/db",
		"GEMINI_API_KEY":     "gemini-key",
		"CHANNEL":            "telegram",
		"TELEGRAM_BOT_TOKEN": "123:abc",
		"TELEGRAM_CHAT_ID":   "987654321",
	}
}

func TestLoad_DefaultChannelIsWhatsApp(t *testing.T) {
	setEnv(t, validEnv())

	cfg, err := Load()
	if err != nil {
		t.Fatalf("esperava sucesso, got: %v", err)
	}
	if cfg.Channel != ChannelWhatsApp {
		t.Errorf("canal padrão esperado whatsapp, got %q", cfg.Channel)
	}
}

func TestLoad_Telegram_NoEvolutionVarsRequired(t *testing.T) {
	setEnv(t, telegramEnv())

	cfg, err := Load()
	if err != nil {
		t.Fatalf("esperava sucesso sem variáveis do WhatsApp, got: %v", err)
	}
	if cfg.Channel != ChannelTelegram || cfg.TelegramBotToken != "123:abc" || cfg.TelegramChatID != 987654321 {
		t.Errorf("config telegram incorreta: %+v", cfg)
	}
}

func TestLoad_Telegram_MissingVars(t *testing.T) {
	for _, key := range []string{"TELEGRAM_BOT_TOKEN", "TELEGRAM_CHAT_ID"} {
		env := telegramEnv()
		env[key] = ""
		setEnv(t, env)
		if _, err := Load(); err == nil {
			t.Errorf("esperava erro sem %s", key)
		}
	}
}

func TestLoad_Telegram_InvalidChatID(t *testing.T) {
	for _, id := range []string{"abc", "-1001234", "0"} {
		env := telegramEnv()
		env["TELEGRAM_CHAT_ID"] = id
		setEnv(t, env)
		if _, err := Load(); err == nil {
			t.Errorf("esperava erro para TELEGRAM_CHAT_ID=%q", id)
		}
	}
}

func TestLoad_InvalidChannel(t *testing.T) {
	env := validEnv()
	env["CHANNEL"] = "sms"
	setEnv(t, env)
	if _, err := Load(); err == nil {
		t.Error("esperava erro para CHANNEL inválido")
	}
}

func TestLoad_WhatsApp_StillRequiresEvolutionVars(t *testing.T) {
	env := validEnv()
	env["OWNER_PHONE"] = ""
	setEnv(t, env)
	if _, err := Load(); err == nil {
		t.Error("esperava erro sem OWNER_PHONE no canal whatsapp")
	}
}

const itemA = "11111111-2222-3333-4444-555555555555"

func openFinanceEnv() map[string]string {
	env := validEnv()
	env["PLUGGY_CLIENT_ID"] = "client-id"
	env["PLUGGY_CLIENT_SECRET"] = "client-secret"
	env["PLUGGY_ITEM_IDS"] = itemA + ", 66666666-7777-8888-9999-000000000000"
	return env
}

func TestLoad_OpenFinanceDisabledByDefault(t *testing.T) {
	setEnv(t, validEnv())
	cfg, err := Load()
	if err != nil || cfg.OpenFinanceEnabled() {
		t.Errorf("Open Finance deveria ficar desligado sem variáveis: %v", err)
	}
}

func TestLoad_OpenFinance_Success(t *testing.T) {
	setEnv(t, openFinanceEnv())
	cfg, err := Load()
	if err != nil {
		t.Fatalf("esperava sucesso, got: %v", err)
	}
	if !cfg.OpenFinanceEnabled() || len(cfg.PluggyItemIDs) != 2 || cfg.PluggyItemIDs[0] != itemA {
		t.Errorf("config incorreta: %+v", cfg)
	}
	if cfg.OpenFinanceSyncInterval.Hours() != 6 || cfg.OpenFinanceLookbackDays != 60 {
		t.Errorf("defaults incorretos: %v %d", cfg.OpenFinanceSyncInterval, cfg.OpenFinanceLookbackDays)
	}
}

func TestLoad_OpenFinance_Invalid(t *testing.T) {
	cases := map[string]func(map[string]string){
		"sem secret":    func(e map[string]string) { delete(e, "PLUGGY_CLIENT_SECRET") },
		"sem itens":     func(e map[string]string) { delete(e, "PLUGGY_ITEM_IDS") },
		"item inválido": func(e map[string]string) { e["PLUGGY_ITEM_IDS"] = "../admin" },
		"intervalo 0":   func(e map[string]string) { e["SYNC_INTERVAL_HOURS"] = "0" },
		"janela 400":    func(e map[string]string) { e["SYNC_LOOKBACK_DAYS"] = "400" },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			env := openFinanceEnv()
			mutate(env)
			for _, k := range []string{"PLUGGY_CLIENT_SECRET", "PLUGGY_ITEM_IDS"} {
				t.Setenv(k, "")
			}
			setEnv(t, env)
			if _, err := Load(); err == nil {
				t.Error("esperava erro de configuração")
			}
		})
	}
}

func TestLoad_DashboardPassword(t *testing.T) {
	for name, tc := range map[string]struct {
		password string
		wantErr  bool
	}{
		"ausente (API desligada)": {"", false},
		"forte":                   {"uma-senha-bem-longa", false},
		"curta":                   {"curta", true},
	} {
		t.Run(name, func(t *testing.T) {
			env := validEnv()
			env["DASHBOARD_PASSWORD"] = tc.password
			setEnv(t, env)
			cfg, err := Load()
			if (err != nil) != tc.wantErr {
				t.Fatalf("err = %v, wantErr = %v", err, tc.wantErr)
			}
			if err == nil && cfg.DashboardPassword != tc.password {
				t.Errorf("senha não carregada")
			}
		})
	}
}

func TestLoad_CoachSettings(t *testing.T) {
	for name, tc := range map[string]struct {
		paid, model string
		want        bool
		wantErr     bool
	}{
		"padrão (bloqueado)": {"", "", false, false},
		"plano pago":         {"true", "gemini-x", true, false},
		"valor inválido":     {"talvez", "", false, true},
	} {
		t.Run(name, func(t *testing.T) {
			env := validEnv()
			env["GEMINI_PAID_PLAN"] = tc.paid
			env["COACH_GEMINI_MODEL"] = tc.model
			setEnv(t, env)
			cfg, err := Load()
			if (err != nil) != tc.wantErr {
				t.Fatalf("err = %v, wantErr = %v", err, tc.wantErr)
			}
			if err == nil && (cfg.GeminiPaidPlan != tc.want || cfg.CoachModel != tc.model) {
				t.Errorf("cfg = %v %q", cfg.GeminiPaidPlan, cfg.CoachModel)
			}
		})
	}
}
