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

type Config struct {
	Port int

	// Channel define por onde o usuário conversa com o assistente: whatsapp (padrão) ou telegram.
	Channel string

	DatabaseURL string

	GeminiAPIKey string

	EvolutionAPIURL   string
	EvolutionInstance string
	EvolutionAPIKey   string
	OwnerPhone        string

	AllowedNumbers map[string]struct{}

	AdminSecret string

	TelegramBotToken string
	TelegramChatID   int64

	// Open Finance (Meu Pluggy) é opcional: sem PLUGGY_CLIENT_ID só o registro manual fica ativo.
	PluggyClientID          string
	PluggyClientSecret      string
	PluggyItemIDs           []string
	OpenFinanceSyncInterval time.Duration
	OpenFinanceLookbackDays int
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

	cfg.Channel = strings.ToLower(strings.TrimSpace(getEnv("CHANNEL", ChannelWhatsApp)))
	cfg.AllowedNumbers = parseAllowedNumbers(getEnv("ALLOWED_NUMBERS", ""))
	cfg.AdminSecret = getEnv("ADMIN_SECRET", "")

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

func loadOpenFinance(cfg *Config, errs *[]error) {
	cfg.PluggyClientID = getEnv("PLUGGY_CLIENT_ID", "")
	cfg.PluggyClientSecret = getEnv("PLUGGY_CLIENT_SECRET", "")
	itemsRaw := getEnv("PLUGGY_ITEM_IDS", "")
	if cfg.PluggyClientID == "" && cfg.PluggyClientSecret == "" && itemsRaw == "" {
		return
	}

	if cfg.PluggyClientID == "" || cfg.PluggyClientSecret == "" || itemsRaw == "" {
		*errs = append(*errs, errors.New("Open Finance incompleto: PLUGGY_CLIENT_ID, PLUGGY_CLIENT_SECRET e PLUGGY_ITEM_IDS são obrigatórias juntas"))
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

func getEnv(key, defaultValue string) string {
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
