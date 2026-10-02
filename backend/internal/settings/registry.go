// Package settings guarda as configurações editáveis pelo dashboard. A ordem de precedência é:
// valor salvo no dashboard > variável de ambiente > padrão.
package settings

import (
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/google/uuid"
)

type Kind string

const (
	KindText   Kind = "text"
	KindInt    Kind = "int"
	KindBool   Kind = "bool"
	KindEnum   Kind = "enum"
	KindList   Kind = "list"   // valores separados por vírgula
	KindSecret Kind = "secret" // nunca volta para a tela
)

// Grupos da página, na ordem em que aparecem.
const (
	GroupDigest   = "digest"
	GroupSync     = "sync"
	GroupPluggy   = "pluggy"
	GroupAI       = "ai"
	GroupTelegram = "telegram"
	GroupWhatsApp = "whatsapp"
)

var Groups = []struct{ ID, Title, Help string }{
	{GroupDigest, "Resumo semanal", "Mensagem de resumo no seu chat, calculada por código (sem IA)."},
	{GroupSync, "Sincronização", "Com que frequência o app busca as transações no banco."},
	{GroupPluggy, "Open Finance (Meu Pluggy)", "Credenciais da aplicação Pluggy e os bancos conectados."},
	{GroupAI, "IA (Gemini)", "Usada para ler notas, extratos e no Coach."},
	{GroupTelegram, "Telegram", "Bot e conta autorizada a falar com ele."},
	{GroupWhatsApp, "WhatsApp (Evolution API)", "Conexão com a Evolution API e números autorizados."},
}

// Def descreve uma configuração. Live: vale na hora; senão, só depois de reiniciar o app.
type Def struct {
	Key      string
	Env      string // nome da variável de ambiente (igual à chave)
	Label    string
	Help     string
	Group    string
	Kind     Kind
	Options  []string // KindEnum
	Default  string
	Live     bool
	Min, Max int // KindInt
	Channel  string
	Validate func(string) error
}

const maxValueLen = 500

var weekdays = []string{"monday", "tuesday", "wednesday", "thursday", "friday", "saturday", "sunday"}

// Defs é o catálogo. Fora dele (porta, banco, senha do dashboard, canal, backup) só o ambiente manda.
var Defs = []Def{
	{Key: "DIGEST_ENABLED", Group: GroupDigest, Label: "Enviar o resumo semanal", Kind: KindBool, Default: "true", Live: true},
	{Key: "DIGEST_WEEKDAY", Group: GroupDigest, Label: "Dia da semana", Kind: KindEnum, Options: weekdays, Default: "monday", Live: true},
	{Key: "DIGEST_HOUR", Group: GroupDigest, Label: "Hora do envio (0 a 23)", Kind: KindInt, Min: 0, Max: 23, Default: "9", Live: true},
	{Key: "DIGEST_TIMEZONE", Group: GroupDigest, Label: "Fuso horário", Help: "Também é o fuso usado nos horários e vencimentos das mensagens.", Kind: KindText, Default: "America/Sao_Paulo", Live: true, Validate: validTimezone},

	{Key: "SYNC_INTERVAL_HOURS", Group: GroupSync, Label: "Sincronizar a cada (horas)", Help: "O Meu Pluggy atualiza os dados cerca de uma vez por dia.", Kind: KindInt, Min: 1, Max: 168, Default: "6", Live: true},
	{Key: "SYNC_LOOKBACK_DAYS", Group: GroupSync, Label: "Dias para trás a cada sincronização", Help: "Máximo 365. É seguro repetir: nada duplica.", Kind: KindInt, Min: 1, Max: 365, Default: "60", Live: true},
	{Key: "OWN_NAMES", Group: GroupSync, Label: "Seus nomes nos extratos", Help: "Como aparecem nos Pix e transferências (separe por vírgula). Pix, TED e DOC com esse nome são entre contas suas e não contam como gasto nem renda.", Kind: KindList, Live: true, Validate: validNames},

	{Key: "PLUGGY_CLIENT_ID", Group: GroupPluggy, Label: "Client ID", Kind: KindText, Live: true, Validate: validToken},
	{Key: "PLUGGY_CLIENT_SECRET", Group: GroupPluggy, Label: "Client Secret", Help: "Trate como senha.", Kind: KindSecret, Live: true, Validate: validToken},
	{Key: "PLUGGY_ITEM_IDS", Group: GroupPluggy, Label: "Item IDs dos bancos", Help: "Um UUID por banco conectado no Meu Pluggy, separados por vírgula.", Kind: KindList, Live: true, Validate: validUUIDs},

	{Key: "GEMINI_API_KEY", Group: GroupAI, Label: "Chave da API do Gemini", Kind: KindSecret, Live: false, Validate: validToken},
	{Key: "GEMINI_PAID_PLAN", Group: GroupAI, Label: "O projeto da chave tem faturamento (plano pago)", Help: "O Coach envia valores e nomes de estabelecimentos ao Gemini. No plano gratuito o Google pode usar esse conteúdo, então só marque se a chave é de um projeto pago. Sem isso, o Coach fica bloqueado.", Kind: KindBool, Default: "false", Live: true},
	{Key: "COACH_GEMINI_MODEL", Group: GroupAI, Label: "Modelo do Coach", Help: "Vazio usa o padrão do app.", Kind: KindText, Live: true, Validate: validToken},

	{Key: "TELEGRAM_BOT_TOKEN", Group: GroupTelegram, Label: "Token do bot", Help: "Criado no @BotFather.", Kind: KindSecret, Channel: "telegram", Validate: validToken},
	{Key: "TELEGRAM_CHAT_ID", Group: GroupTelegram, Label: "Seu ID numérico", Help: "Descubra no @userinfobot. Só esse usuário é atendido.", Kind: KindText, Channel: "telegram", Validate: validPositiveInt},

	{Key: "EVOLUTION_API_KEY", Group: GroupWhatsApp, Label: "Chave da Evolution API", Kind: KindSecret, Channel: "whatsapp", Validate: validToken},
	{Key: "EVOLUTION_API_URL", Group: GroupWhatsApp, Label: "Endereço da Evolution API", Kind: KindText, Channel: "whatsapp", Validate: validURL},
	{Key: "EVOLUTION_INSTANCE", Group: GroupWhatsApp, Label: "Nome da instância", Kind: KindText, Channel: "whatsapp", Validate: validToken},
	{Key: "OWNER_PHONE", Group: GroupWhatsApp, Label: "Seu telefone (com DDI e DDD)", Kind: KindText, Channel: "whatsapp", Validate: validDigits},
	{Key: "ALLOWED_NUMBERS", Group: GroupWhatsApp, Label: "Números autorizados", Help: "Separados por vírgula.", Kind: KindList, Channel: "whatsapp", Validate: validDigitList},
}

func init() {
	for i := range Defs {
		Defs[i].Env = Defs[i].Key
	}
}

// sensitiveKeys, além dos segredos, são o que redireciona dados ou abre acesso: trocar o Item ID, o ID autorizado no
// Telegram ou o endereço da Evolution API entregaria os dados (ou o controle do bot) a outra pessoa.
var sensitiveKeys = map[string]bool{
	"PLUGGY_CLIENT_ID": true, "PLUGGY_ITEM_IDS": true, "TELEGRAM_CHAT_ID": true,
	"EVOLUTION_API_URL": true, "EVOLUTION_INSTANCE": true, "OWNER_PHONE": true, "ALLOWED_NUMBERS": true,
}

// Sensitive diz se mudar (ou restaurar) a configuração exige confirmar a senha do dashboard.
func (d Def) Sensitive() bool { return d.Kind == KindSecret || sensitiveKeys[d.Key] }

// Lookup devolve a definição da chave.
func Lookup(key string) (Def, bool) {
	for _, d := range Defs {
		if d.Key == key {
			return d, true
		}
	}
	return Def{}, false
}

// Check valida o valor para a definição. Valor vazio é aceito (volta ao ambiente/padrão) apenas pelo Reset; aqui, vazio vale
// para campos de texto/lista opcionais, e as regras por tipo decidem o resto.
func (d Def) Check(value string) error {
	if len(value) > maxValueLen {
		return fmt.Errorf("%s: valor longo demais", d.Label)
	}
	for _, r := range value {
		if unicode.IsControl(r) {
			return fmt.Errorf("%s: caracteres inválidos", d.Label)
		}
	}
	switch d.Kind {
	case KindBool:
		if value != "true" && value != "false" {
			return fmt.Errorf("%s: use ligado ou desligado", d.Label)
		}
	case KindInt:
		n, err := strconv.Atoi(value)
		if err != nil || n < d.Min || n > d.Max {
			return fmt.Errorf("%s: informe um número entre %d e %d", d.Label, d.Min, d.Max)
		}
	case KindEnum:
		for _, o := range d.Options {
			if o == value {
				return nil
			}
		}
		return fmt.Errorf("%s: valor fora das opções", d.Label)
	}
	if d.Validate != nil && value != "" {
		if err := d.Validate(value); err != nil {
			return fmt.Errorf("%s: %w", d.Label, err)
		}
	}
	return nil
}

// SplitList separa uma lista por vírgula, sem vazios nem espaços sobrando.
func SplitList(raw string) []string {
	var out []string
	for _, p := range strings.Split(raw, ",") {
		if p = strings.Join(strings.Fields(p), " "); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func validTimezone(v string) error {
	if _, err := time.LoadLocation(v); err != nil {
		return errors.New("fuso desconhecido (ex.: America/Sao_Paulo)")
	}
	return nil
}

func validNames(v string) error {
	for _, n := range SplitList(v) {
		if len([]rune(n)) < 5 {
			return errors.New("nome curto demais: nomes pequenos casariam com descrições de outras pessoas")
		}
	}
	return nil
}

func validToken(v string) error {
	if strings.ContainsAny(v, " \t") {
		return errors.New("não pode ter espaços")
	}
	return nil
}

func validUUIDs(v string) error {
	for _, id := range SplitList(v) {
		if _, err := uuid.Parse(id); err != nil {
			return fmt.Errorf("%q não é um UUID", id)
		}
	}
	return nil
}

func validPositiveInt(v string) error {
	if n, err := strconv.ParseInt(v, 10, 64); err != nil || n <= 0 {
		return errors.New("deve ser um número positivo")
	}
	return nil
}

func validURL(v string) error {
	u, err := url.Parse(v)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return errors.New("use um endereço http:// ou https://")
	}
	return nil
}

func validDigits(v string) error {
	for _, r := range v {
		if r < '0' || r > '9' {
			return errors.New("só números")
		}
	}
	return nil
}

func validDigitList(v string) error {
	for _, n := range SplitList(v) {
		if err := validDigits(n); err != nil {
			return err
		}
	}
	return nil
}
