package ports

import (
	"context"
	"time"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain"
	"github.com/google/uuid"
)

// ExternalTransaction é uma transação vinda do Open Finance já normalizada para o
// vocabulário do app. O adapter descarta o que não deve ser contado (pagamento de
// fatura, transferência entre contas próprias), então tudo aqui vira lançamento.
type ExternalTransaction struct {
	ID            string
	AccountID     string // ID da conta na origem (ExternalAccount.ID)
	Date          time.Time
	Description   string
	RawInput      string
	Amount        float64 // sempre positivo
	Kind          domain.PurchaseKind
	Direction     domain.TransferDirection // só para KindTransfer
	Category      domain.Category
	PaymentMethod domain.PaymentMethod
	Pending       bool // compra ainda na fatura aberta do cartão
}

// ExternalAccount é uma conta ou cartão do Open Finance. Só guarda o necessário:
// nunca CPF, nome do titular nem o número completo.
type ExternalAccount struct {
	ID                   string
	ItemID               string
	Type                 string // "BANK" ou "CREDIT"
	Name                 string
	Last4                string
	Balance              float64
	CreditLimit          *float64 // só cartões
	AvailableCreditLimit *float64 // só cartões

	// Opcionais: dependem do que o banco informa.
	Brand          string     // bandeira do cartão
	CloseDate      *time.Time // fechamento da fatura
	DueDate        *time.Time // vencimento da fatura
	MinimumPayment *float64   // pagamento mínimo da fatura
	AutoInvested   *float64   // saldo aplicado automaticamente (conta corrente)
	InstitutionID  *uuid.UUID // preenchido na sincronização, depois de salvar a instituição
}

// ExternalInstitution é o banco da conexão (conector do Pluggy). Color é hexadecimal de 6
// dígitos sem "#", já validado pelo adapter; vazio quando o conector não informa.
type ExternalInstitution struct {
	Name     string
	Color    string
	ImageURL string
}

// LogoFetcher baixa o logo de uma instituição de forma segura (só https, sem rede interna,
// com limite de tamanho e de tipo). O erro nunca deve derrubar a sincronização.
type LogoFetcher interface {
	Fetch(ctx context.Context, imageURL string) (data []byte, mime string, err error)
}

type ItemData struct {
	Institution  *ExternalInstitution // nil quando o conector não pôde ser lido
	Accounts     []ExternalAccount
	Transactions []ExternalTransaction
}

type OpenFinanceProvider interface {
	// FetchItem retorna as contas e as transações de todas as contas e cartões do item desde "from".
	FetchItem(ctx context.Context, itemID string, from time.Time) (ItemData, error)
	// FetchInvestments retorna as posições de investimento do item (vazio se não houver).
	FetchInvestments(ctx context.Context, itemID string) ([]ExternalInvestment, error)
}

// ExternalInvestment é uma posição de investimento (CDB, fundo, ação...). Só guarda o
// necessário: nunca titular, CNPJ do emissor nem o número da posição.
type ExternalInvestment struct {
	ID      string
	ItemID  string
	Type    string // FIXED_INCOME, EQUITY, MUTUAL_FUND, SECURITY, ETF, COE ou OTHER
	Subtype string // CDB, LCI, ...
	Name    string
	Balance float64 // saldo líquido atual
	Amount  float64 // valor bruto

	Movements       []ExternalMovement
	MovementsFailed bool // a leitura das movimentações falhou (a posição em si foi lida)
}

// ExternalMovement é uma movimentação da posição. Amount tem sinal: positivo entra na
// posição (aplicação), negativo sai (resgate, taxa).
type ExternalMovement struct {
	ID     string
	Day    time.Time
	Amount float64
}
