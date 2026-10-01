package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

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
}

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

	if err := errors.Join(errs...); err != nil {
		return nil, fmt.Errorf("configuração inválida:\n%w", err)
	}

	return cfg, nil
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
