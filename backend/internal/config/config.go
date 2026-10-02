package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/joho/godotenv"
)

const (
	ChannelWhatsApp = "whatsapp"
	ChannelTelegram = "telegram"
)

const minDashboardPasswordLen = 12

type Config struct {
	Port int

	// Channel define por onde o usuário conversa com o assistente: whatsapp (padrão) ou telegram.
	Channel string

	DatabaseURL string

	GeminiAPIKey string
	// GeminiPaidPlan é a declaração de que o projeto da chave tem faturamento (serviços pagos). O Coach, que
	// envia dados financeiros ao Gemini, só funciona com ela: no plano grátis o Google pode usar e revisar o conteúdo.
	GeminiPaidPlan bool
	// Resumo semanal (calculado por código, sem IA) enviado ao dono no canal de conversa.
	DigestEnabled  bool
	DigestWeekday  time.Weekday
	DigestHour     int
	DigestLocation *time.Location

	// CoachModel troca o modelo do Coach; vazio usa o padrão do cliente.
	CoachModel string

	EvolutionAPIURL   string
	EvolutionInstance string
	EvolutionAPIKey   string
	OwnerPhone        string

	AllowedNumbers map[string]struct{}

	AdminSecret string

	// DashboardPassword libera a API do dashboard (/api). Vazia, a API não é montada.
	DashboardPassword string

	TelegramBotToken string
	TelegramChatID   int64

	// Open Finance (Meu Pluggy) é opcional: sem PLUGGY_CLIENT_ID só o registro manual fica ativo.
	PluggyClientID          string
	PluggyClientSecret      string
	PluggyItemIDs           []string
	OpenFinanceSyncInterval time.Duration
	OpenFinanceLookbackDays int
	OwnNames                []string // seus nomes, para ignorar Pix e transferências entre contas suas
}

func (c *Config) OpenFinanceEnabled() bool { return c.PluggyClientID != "" }

// Load lê a configuração. Os valores de overrides (salvos na página de configurações) valem mais que o ambiente;
// nil usa só o ambiente (e o .env, se existir).
func Load(overrides map[string]string) (*Config, error) {
	if err := godotenv.Load(); err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("erro ao carregar .env: %w", err)
	}

	l := &loader{overrides: overrides}
	cfg := &Config{}
	l.app(cfg)
	l.digest(cfg)
	l.channel(cfg)
	l.openFinance(cfg)

	if err := errors.Join(l.errs...); err != nil {
		return nil, fmt.Errorf("configuração inválida:\n%w", err)
	}
	return cfg, nil
}

// loader lê cada variável (override > ambiente > padrão) e acumula os erros, para o usuário ver todos de uma vez.
type loader struct {
	overrides map[string]string
	errs      []error
}

func (l *loader) get(key, defaultValue string) string {
	if value, ok := l.overrides[key]; ok {
		return value
	}
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return defaultValue
}

func (l *loader) fail(format string, args ...any) {
	l.errs = append(l.errs, fmt.Errorf(format, args...))
}

func (l *loader) require(key string) string {
	value := l.get(key, "")
	if value == "" {
		l.fail("%s é obrigatória", key)
	}
	return value
}

// boolean lê um booleano com padrão; vazio vale o padrão.
func (l *loader) boolean(key string, def bool) bool {
	raw := strings.TrimSpace(l.get(key, ""))
	if raw == "" {
		return def
	}
	v, err := strconv.ParseBool(raw)
	if err != nil {
		l.fail("%s inválida: %q — use true ou false", key, raw)
	}
	return v
}

// app lê o que é do aplicativo em si: porta, banco, Gemini, dashboard.
func (l *loader) app(cfg *Config) {
	portStr := l.get("PORT", "8080")
	port, err := strconv.Atoi(portStr)
	if err != nil {
		l.fail("PORT inválida: %q — deve ser um número", portStr)
	}
	cfg.Port = port

	cfg.DatabaseURL = l.get("DATABASE_URL", "")
	if cfg.DatabaseURL == "" {
		l.fail("DATABASE_URL é obrigatória")
	}
	cfg.GeminiAPIKey = l.get("GEMINI_API_KEY", "")
	if cfg.GeminiAPIKey == "" {
		l.fail("GEMINI_API_KEY é obrigatória")
	}
	cfg.GeminiPaidPlan = l.boolean("GEMINI_PAID_PLAN", false)
	cfg.CoachModel = strings.TrimSpace(l.get("COACH_GEMINI_MODEL", ""))

	cfg.AdminSecret = l.get("ADMIN_SECRET", "")
	cfg.DashboardPassword = l.get("DASHBOARD_PASSWORD", "")
	if cfg.DashboardPassword != "" && len(cfg.DashboardPassword) < minDashboardPasswordLen {
		l.fail("DASHBOARD_PASSWORD muito curta: use ao menos %d caracteres", minDashboardPasswordLen)
	}
}

// channel lê o canal de conversa (WhatsApp ou Telegram) e exige só o que o canal escolhido usa.
func (l *loader) channel(cfg *Config) {
	cfg.Channel = strings.ToLower(strings.TrimSpace(l.get("CHANNEL", ChannelWhatsApp)))
	cfg.AllowedNumbers = parseAllowedNumbers(l.get("ALLOWED_NUMBERS", ""))

	switch cfg.Channel {
	case ChannelWhatsApp:
		cfg.EvolutionAPIURL = l.get("EVOLUTION_API_URL", "http://evolution:8082")
		cfg.EvolutionInstance = l.require("EVOLUTION_INSTANCE")
		cfg.EvolutionAPIKey = l.require("EVOLUTION_API_KEY")
		cfg.OwnerPhone = l.require("OWNER_PHONE")
	case ChannelTelegram:
		cfg.TelegramBotToken = l.require("TELEGRAM_BOT_TOKEN")
		if raw := l.require("TELEGRAM_CHAT_ID"); raw != "" {
			chatID, err := strconv.ParseInt(raw, 10, 64)
			if err != nil || chatID <= 0 {
				l.fail("TELEGRAM_CHAT_ID inválido: deve ser o ID numérico da sua conta (use @userinfobot)")
			}
			cfg.TelegramChatID = chatID
		}
	default:
		l.fail("CHANNEL inválido: %q — use %q ou %q", cfg.Channel, ChannelWhatsApp, ChannelTelegram)
	}
}

var weekdays = map[string]time.Weekday{
	"sunday": time.Sunday, "monday": time.Monday, "tuesday": time.Tuesday, "wednesday": time.Wednesday,
	"thursday": time.Thursday, "friday": time.Friday, "saturday": time.Saturday,
}

// digest lê DIGEST_ENABLED (padrão true), DIGEST_WEEKDAY (monday..sunday, padrão monday), DIGEST_HOUR (0-23, padrão 9) e
// DIGEST_TIMEZONE (padrão America/Sao_Paulo).
func (l *loader) digest(cfg *Config) {
	cfg.DigestEnabled = l.boolean("DIGEST_ENABLED", true)

	day := strings.ToLower(strings.TrimSpace(l.get("DIGEST_WEEKDAY", "monday")))
	if day == "" {
		day = "monday"
	}
	weekday, ok := weekdays[day]
	if !ok {
		l.fail("DIGEST_WEEKDAY inválido: %q — use monday, tuesday, wednesday, thursday, friday, saturday ou sunday", day)
	}
	cfg.DigestWeekday = weekday

	hourStr := strings.TrimSpace(l.get("DIGEST_HOUR", "9"))
	if hourStr == "" {
		hourStr = "9"
	}
	hour, err := strconv.Atoi(hourStr)
	if err != nil || hour < 0 || hour > 23 {
		l.fail("DIGEST_HOUR inválida: %q — use um inteiro de 0 a 23", hourStr)
	}
	cfg.DigestHour = hour

	zone := strings.TrimSpace(l.get("DIGEST_TIMEZONE", "America/Sao_Paulo"))
	if zone == "" {
		zone = "America/Sao_Paulo"
	}
	if cfg.DigestLocation, err = time.LoadLocation(zone); err != nil {
		l.fail("DIGEST_TIMEZONE inválido: %q", zone)
	}
}

// openFinance lê as credenciais do Pluggy (opcionais, mas só valem juntas), o intervalo e a janela da sincronização.
func (l *loader) openFinance(cfg *Config) {
	cfg.PluggyClientID = l.get("PLUGGY_CLIENT_ID", "")
	cfg.PluggyClientSecret = l.get("PLUGGY_CLIENT_SECRET", "")
	itemsRaw := l.get("PLUGGY_ITEM_IDS", "")
	if cfg.PluggyClientID == "" && cfg.PluggyClientSecret == "" && itemsRaw == "" {
		return
	}
	if cfg.PluggyClientID == "" || cfg.PluggyClientSecret == "" || itemsRaw == "" {
		l.fail("configuração do Open Finance incompleta: PLUGGY_CLIENT_ID, PLUGGY_CLIENT_SECRET e PLUGGY_ITEM_IDS são obrigatórias juntas")
		return
	}

	for _, id := range strings.Split(itemsRaw, ",") {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		if _, err := uuid.Parse(id); err != nil {
			l.fail("PLUGGY_ITEM_IDS contém um itemId inválido: %q (deve ser um UUID)", id)
			continue
		}
		cfg.PluggyItemIDs = append(cfg.PluggyItemIDs, id)
	}

	for _, name := range strings.Split(l.get("OWN_NAMES", ""), ",") {
		if name = strings.Join(strings.Fields(name), " "); name != "" {
			cfg.OwnNames = append(cfg.OwnNames, name)
		}
	}

	hours, err := strconv.Atoi(l.get("SYNC_INTERVAL_HOURS", "6"))
	if err != nil || hours < 1 {
		l.fail("SYNC_INTERVAL_HOURS inválida: deve ser um inteiro >= 1")
	}
	cfg.OpenFinanceSyncInterval = time.Duration(hours) * time.Hour

	// O Pluggy só guarda os últimos 12 meses.
	cfg.OpenFinanceLookbackDays, err = strconv.Atoi(l.get("SYNC_LOOKBACK_DAYS", "60"))
	if err != nil || cfg.OpenFinanceLookbackDays < 1 || cfg.OpenFinanceLookbackDays > 365 {
		l.fail("SYNC_LOOKBACK_DAYS inválida: deve ser um inteiro entre 1 e 365")
	}
}

// Bootstrap carrega o .env e devolve o que é preciso para abrir o banco e ler as configurações salvas.
func Bootstrap() (databaseURL, secretKey string, err error) {
	if err := godotenv.Load(); err != nil && !os.IsNotExist(err) {
		return "", "", fmt.Errorf("erro ao carregar .env: %w", err)
	}
	key, err := ReadSecretKey(os.Getenv("APP_SECRET_KEY"), os.Getenv("APP_SECRET_KEY_FILE"))
	if err != nil {
		return "", "", err
	}
	return os.Getenv("DATABASE_URL"), key, nil
}

// ReadSecretKey devolve a chave mestra dos segredos. Prefere o arquivo (APP_SECRET_KEY_FILE, ex.: um Docker secret): ele
// não aparece em `docker inspect` nem no ambiente do processo. Sem arquivo, usa a variável APP_SECRET_KEY.
func ReadSecretKey(envValue, file string) (string, error) {
	if file == "" {
		return envValue, nil
	}
	raw, err := os.ReadFile(file)
	if err != nil {
		return "", fmt.Errorf("APP_SECRET_KEY_FILE ilegível: %w", err)
	}
	key := strings.TrimSpace(string(raw))
	if key == "" {
		return "", errors.New("APP_SECRET_KEY_FILE está vazio")
	}
	return key, nil
}

func parseAllowedNumbers(raw string) map[string]struct{} {
	allowed := make(map[string]struct{})
	for _, n := range strings.Split(raw, ",") {
		n = strings.TrimSpace(n)
		if n == "" {
			continue
		}
		if !strings.Contains(n, "@") {
			n = n + "@s.whatsapp.net"
		}
		allowed[n] = struct{}{}
	}
	return allowed
}
