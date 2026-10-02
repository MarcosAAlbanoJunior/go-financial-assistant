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

func Load() (*Config, error) {

	if err := godotenv.Load(); err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("erro ao carregar .env: %w", err)
	}

	cfg := &Config{}
	var errs []error

	portStr := getEnv("PORT", "8080")
	port, err := strconv.Atoi(portStr)
	if err != nil {
		errs = append(errs, fmt.Errorf("PORT inválida: %q — deve ser um número", portStr))
	}
	cfg.Port = port

	cfg.DatabaseURL = getEnv("DATABASE_URL", "")
	if cfg.DatabaseURL == "" {
		errs = append(errs, errors.New("DATABASE_URL é obrigatória"))
	}

	cfg.GeminiAPIKey = getEnv("GEMINI_API_KEY", "")
	if cfg.GeminiAPIKey == "" {
		errs = append(errs, errors.New("GEMINI_API_KEY é obrigatória"))
	}

	paid := strings.TrimSpace(getEnv("GEMINI_PAID_PLAN", ""))
	if paid == "" {
		paid = "false"
	}
	if cfg.GeminiPaidPlan, err = strconv.ParseBool(paid); err != nil {
		errs = append(errs, fmt.Errorf("GEMINI_PAID_PLAN inválida: %q — use true ou false", paid))
	}
	cfg.CoachModel = strings.TrimSpace(getEnv("COACH_GEMINI_MODEL", ""))
	loadDigest(cfg, &errs)

	cfg.Channel = strings.ToLower(strings.TrimSpace(getEnv("CHANNEL", ChannelWhatsApp)))
	cfg.AllowedNumbers = parseAllowedNumbers(getEnv("ALLOWED_NUMBERS", ""))
	cfg.AdminSecret = getEnv("ADMIN_SECRET", "")
	cfg.DashboardPassword = getEnv("DASHBOARD_PASSWORD", "")
	if cfg.DashboardPassword != "" && len(cfg.DashboardPassword) < minDashboardPasswordLen {
		errs = append(errs, fmt.Errorf("DASHBOARD_PASSWORD muito curta: use ao menos %d caracteres", minDashboardPasswordLen))
	}

	switch cfg.Channel {
	case ChannelWhatsApp:
		cfg.EvolutionAPIURL = getEnv("EVOLUTION_API_URL", "http://evolution:8082")
		cfg.EvolutionInstance = requireEnv("EVOLUTION_INSTANCE", &errs)
		cfg.EvolutionAPIKey = requireEnv("EVOLUTION_API_KEY", &errs)
		cfg.OwnerPhone = requireEnv("OWNER_PHONE", &errs)
	case ChannelTelegram:
		cfg.TelegramBotToken = requireEnv("TELEGRAM_BOT_TOKEN", &errs)
		chatIDStr := requireEnv("TELEGRAM_CHAT_ID", &errs)
		if chatIDStr != "" {
			chatID, err := strconv.ParseInt(chatIDStr, 10, 64)
			if err != nil || chatID <= 0 {
				errs = append(errs, errors.New("TELEGRAM_CHAT_ID inválido: deve ser o ID numérico da sua conta (use @userinfobot)"))
			}
			cfg.TelegramChatID = chatID
		}
	default:
		errs = append(errs, fmt.Errorf("CHANNEL inválido: %q — use %q ou %q", cfg.Channel, ChannelWhatsApp, ChannelTelegram))
	}

	loadOpenFinance(cfg, &errs)

	if err := errors.Join(errs...); err != nil {
		return nil, fmt.Errorf("configuração inválida:\n%w", err)
	}

	return cfg, nil
}

var weekdays = map[string]time.Weekday{
	"sunday": time.Sunday, "monday": time.Monday, "tuesday": time.Tuesday, "wednesday": time.Wednesday,
	"thursday": time.Thursday, "friday": time.Friday, "saturday": time.Saturday,
}

// loadDigest lê DIGEST_ENABLED (padrão true), DIGEST_WEEKDAY (monday..sunday, padrão monday), DIGEST_HOUR (0-23,
// padrão 9) e DIGEST_TIMEZONE (padrão America/Sao_Paulo).
func loadDigest(cfg *Config, errs *[]error) {
	enabled := strings.TrimSpace(getEnv("DIGEST_ENABLED", ""))
	if enabled == "" {
		enabled = "true"
	}
	var err error
	if cfg.DigestEnabled, err = strconv.ParseBool(enabled); err != nil {
		*errs = append(*errs, fmt.Errorf("DIGEST_ENABLED inválida: %q — use true ou false", enabled))
	}

	day := strings.ToLower(strings.TrimSpace(getEnv("DIGEST_WEEKDAY", "monday")))
	if day == "" {
		day = "monday"
	}
	weekday, ok := weekdays[day]
	if !ok {
		*errs = append(*errs, fmt.Errorf("DIGEST_WEEKDAY inválido: %q — use monday, tuesday, wednesday, thursday, friday, saturday ou sunday", day))
	}
	cfg.DigestWeekday = weekday

	hourStr := strings.TrimSpace(getEnv("DIGEST_HOUR", "9"))
	if hourStr == "" {
		hourStr = "9"
	}
	hour, err := strconv.Atoi(hourStr)
	if err != nil || hour < 0 || hour > 23 {
		*errs = append(*errs, fmt.Errorf("DIGEST_HOUR inválida: %q — use um inteiro de 0 a 23", hourStr))
	}
	cfg.DigestHour = hour

	zone := strings.TrimSpace(getEnv("DIGEST_TIMEZONE", "America/Sao_Paulo"))
	if zone == "" {
		zone = "America/Sao_Paulo"
	}
	if cfg.DigestLocation, err = time.LoadLocation(zone); err != nil {
		*errs = append(*errs, fmt.Errorf("DIGEST_TIMEZONE inválido: %q", zone))
	}
}

func loadOpenFinance(cfg *Config, errs *[]error) {
	cfg.PluggyClientID = getEnv("PLUGGY_CLIENT_ID", "")
	cfg.PluggyClientSecret = getEnv("PLUGGY_CLIENT_SECRET", "")
	itemsRaw := getEnv("PLUGGY_ITEM_IDS", "")
	if cfg.PluggyClientID == "" && cfg.PluggyClientSecret == "" && itemsRaw == "" {
		return
	}

	if cfg.PluggyClientID == "" || cfg.PluggyClientSecret == "" || itemsRaw == "" {
		*errs = append(*errs, errors.New("configuração do Open Finance incompleta: PLUGGY_CLIENT_ID, PLUGGY_CLIENT_SECRET e PLUGGY_ITEM_IDS são obrigatórias juntas"))
		return
	}

	for _, id := range strings.Split(itemsRaw, ",") {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		if _, err := uuid.Parse(id); err != nil {
			*errs = append(*errs, fmt.Errorf("PLUGGY_ITEM_IDS contém um itemId inválido: %q (deve ser um UUID)", id))
			continue
		}
		cfg.PluggyItemIDs = append(cfg.PluggyItemIDs, id)
	}

	for _, name := range strings.Split(getEnv("OWN_NAMES", ""), ",") {
		if name = strings.Join(strings.Fields(name), " "); name != "" {
			cfg.OwnNames = append(cfg.OwnNames, name)
		}
	}

	hours, err := strconv.Atoi(getEnv("SYNC_INTERVAL_HOURS", "6"))
	if err != nil || hours < 1 {
		*errs = append(*errs, errors.New("SYNC_INTERVAL_HOURS inválida: deve ser um inteiro >= 1"))
	}
	cfg.OpenFinanceSyncInterval = time.Duration(hours) * time.Hour

	// O Pluggy só guarda os últimos 12 meses.
	cfg.OpenFinanceLookbackDays, err = strconv.Atoi(getEnv("SYNC_LOOKBACK_DAYS", "60"))
	if err != nil || cfg.OpenFinanceLookbackDays < 1 || cfg.OpenFinanceLookbackDays > 365 {
		*errs = append(*errs, errors.New("SYNC_LOOKBACK_DAYS inválida: deve ser um inteiro entre 1 e 365"))
	}
}

func requireEnv(key string, errs *[]error) string {
	value := getEnv(key, "")
	if value == "" {
		*errs = append(*errs, fmt.Errorf("%s é obrigatória", key))
	}
	return value
}

// overrides são os valores salvos na página de configurações: valem mais que o ambiente.
var overrides map[string]string

// SetOverrides define os valores salvos no dashboard, que Load usa antes do ambiente.
func SetOverrides(values map[string]string) { overrides = values }

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

func getEnv(key, defaultValue string) string {
	if value, ok := overrides[key]; ok {
		return value
	}
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return defaultValue
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
