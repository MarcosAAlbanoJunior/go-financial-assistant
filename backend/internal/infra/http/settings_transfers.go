package httpserver

import (
	"context"
	"net/http"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain"
	"github.com/google/uuid"
)

type transferExample struct {
	Description string  `json:"description"`
	Kind        string  `json:"kind"`
	Amount      float64 `json:"amount"`
}

type ownTransfers struct {
	Count    int               `json:"count"`
	Expense  float64           `json:"expense"`
	Income   float64           `json:"income"`
	Examples []transferExample `json:"examples"`
}

// findOwnTransfers procura, entre o que já foi gravado, o que é Pix/TED/DOC com os nomes atuais.
func (a *api) findOwnTransfers(ctx context.Context) (ownTransfers, []uuid.UUID, error) {
	var out ownTransfers
	matcher := domain.NewOwnTransferMatcher(a.settings.Service.List("OWN_NAMES"))
	cands, err := a.settings.Cleaner.OwnTransferCandidates(ctx)
	if err != nil {
		return out, nil, err
	}
	var ids []uuid.UUID
	for _, c := range cands {
		if !matcher.Match(c.Description) {
			continue
		}
		ids = append(ids, c.PaymentID)
		out.Count++
		if c.Kind == "EXPENSE" {
			out.Expense += c.Amount
		} else {
			out.Income += c.Amount
		}
		if len(out.Examples) < maxExamples {
			out.Examples = append(out.Examples, transferExample{Description: c.Description, Kind: c.Kind, Amount: c.Amount})
		}
	}
	return out, ids, nil
}

// applyOwnTransfers cancela as transferências entre contas suas já gravadas (calculadas de novo no servidor, não vindas da tela).
func (a *api) applyOwnTransfers(w http.ResponseWriter, r *http.Request) {
	_, ids, err := a.findOwnTransfers(r.Context())
	if err != nil {
		a.fail(w, "procurar transferências próprias", err)
		return
	}
	var n int64
	if len(ids) > 0 {
		if n, err = a.settings.Cleaner.CancelPayments(r.Context(), ids); err != nil {
			a.fail(w, "cancelar transferências próprias", err)
			return
		}
	}
	a.logger.Info("transferências entre contas próprias canceladas", "count", n)
	writeJSON(w, http.StatusOK, map[string]any{"cancelled": n})
}
