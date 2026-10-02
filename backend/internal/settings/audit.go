package settings

import (
	"context"
	"time"
)

// AuditEntry registra uma alteração, sem o valor: pode ser um segredo.
type AuditEntry struct {
	At        time.Time
	Action    string // "set" ou "reset"
	Key       string
	Sensitive bool
	IP        string
}

// AuditLog guarda o histórico de alterações das configurações.
type AuditLog interface {
	RecordAudit(ctx context.Context, e AuditEntry) error
	RecentAudit(ctx context.Context, limit int) ([]AuditEntry, error)
}
