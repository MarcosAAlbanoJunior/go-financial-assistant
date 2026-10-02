package httpserver

import (
	"net/http"
	"time"

	"github.com/google/uuid"
)

func (a *api) accounts(w http.ResponseWriter, r *http.Request) {
	accounts, err := a.reader.Accounts(r.Context())
	if err != nil {
		a.fail(w, "contas", err)
		return
	}
	type item struct {
		ID                   uuid.UUID `json:"id"`
		Type                 string    `json:"type"`
		Name                 string    `json:"name"`
		Last4                string    `json:"last4"`
		Balance              float64   `json:"balance"`
		CreditLimit          *float64  `json:"creditLimit"`
		AvailableCreditLimit *float64  `json:"availableCreditLimit"`
		UpdatedAt            time.Time `json:"updatedAt"`
	}
	out := make([]item, len(accounts))
	for i, acc := range accounts {
		out[i] = item{acc.ID, acc.Type, acc.Name, acc.Last4, acc.Balance, acc.CreditLimit, acc.AvailableCreditLimit, acc.UpdatedAt}
	}
	writeJSON(w, http.StatusOK, out)
}

func (a *api) registerAccounts(rt routes) {
	rt.mux.Handle("GET /api/accounts", rt.protected(a.accounts))
}
