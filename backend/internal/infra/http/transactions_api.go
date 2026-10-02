package httpserver

import (
	"net/http"
	"strconv"
	"time"
	"unicode/utf8"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain"
	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain/ports"
	"github.com/google/uuid"
)

// filterParams lê os filtros comuns da lista e dos grupos de transações, validando cada um.
func (a *api) filterParams(w http.ResponseWriter, r *http.Request) (ports.TransactionFilter, bool) {
	q := r.URL.Query()
	f := ports.TransactionFilter{Limit: defaultPageSize, Search: q.Get("q")}

	month, ok := a.monthParam(w, r, "month", false)
	if !ok {
		return f, false
	}
	if !month.IsZero() {
		f.Month = &month
	}
	if raw := q.Get("day"); raw != "" {
		day, err := time.Parse(time.DateOnly, raw)
		if err != nil {
			writeError(w, http.StatusBadRequest, "day deve estar no formato AAAA-MM-DD")
			return f, false
		}
		f.Day = &day
	}
	if f.Kind, ok = enumParam(w, q.Get("kind"), "kind", kinds); !ok {
		return f, false
	}
	if f.Category, ok = enumParam(w, q.Get("category"), "category", categories); !ok {
		return f, false
	}
	if f.PaymentMethod, ok = enumParam(w, q.Get("payment_method"), "payment_method", paymentMethods); !ok {
		return f, false
	}
	if raw := q.Get("account"); raw != "" {
		id, err := uuid.Parse(raw)
		if err != nil {
			writeError(w, http.StatusBadRequest, "account inválido")
			return f, false
		}
		f.AccountID = &id
	}
	if utf8.RuneCountInString(f.Search) > maxSearchLen {
		writeError(w, http.StatusBadRequest, "q muito longo")
		return f, false
	}
	return f, true
}

// transactionGroups: totais da lista de transações por categoria ou por dia, sob os mesmos filtros.
func (a *api) transactionGroups(w http.ResponseWriter, r *http.Request) {
	f, ok := a.filterParams(w, r)
	if !ok {
		return
	}
	by := ports.GroupBy(r.URL.Query().Get("by"))
	if by != ports.GroupByCategory && by != ports.GroupByDay {
		writeError(w, http.StatusBadRequest, "by deve ser category ou day")
		return
	}
	groups, err := a.reader.TransactionGroups(r.Context(), f, by)
	if err != nil {
		a.fail(w, "grupos de transações", err)
		return
	}
	type item struct {
		Key      string  `json:"key"`
		Label    string  `json:"label"`
		Count    int     `json:"count"`
		Expense  float64 `json:"expense"`
		Income   float64 `json:"income"`
		Transfer float64 `json:"transfer"`
	}
	out := make([]item, len(groups))
	for i, g := range groups {
		label := g.Key
		if by == ports.GroupByCategory {
			label = domain.Category(g.Key).Label()
		}
		out[i] = item{g.Key, label, g.Count, g.Expense, g.Income, g.Transfer}
	}
	writeJSON(w, http.StatusOK, out)
}

func (a *api) transactions(w http.ResponseWriter, r *http.Request) {
	f, ok := a.filterParams(w, r)
	if !ok {
		return
	}
	q := r.URL.Query()
	page := 1
	if raw := q.Get("page"); raw != "" {
		var err error
		if page, err = strconv.Atoi(raw); err != nil || page < 1 || page > 10000 {
			writeError(w, http.StatusBadRequest, "page inválido")
			return
		}
	}
	if raw := q.Get("limit"); raw != "" {
		var err error
		if f.Limit, err = strconv.Atoi(raw); err != nil || f.Limit < 1 || f.Limit > maxPageSize {
			writeError(w, http.StatusBadRequest, "limit deve estar entre 1 e 100")
			return
		}
	}
	f.Offset = (page - 1) * f.Limit

	txs, total, err := a.reader.Transactions(r.Context(), f)
	if err != nil {
		a.fail(w, "transações", err)
		return
	}
	type item struct {
		ID                uuid.UUID  `json:"id"`
		Date              string     `json:"date"`
		Description       string     `json:"description"`
		Category          string     `json:"category"`
		CategoryLabel     string     `json:"categoryLabel"`
		PaymentMethod     string     `json:"paymentMethod"`
		PaymentLabel      string     `json:"paymentMethodLabel"`
		Kind              string     `json:"kind"`
		TransferDirection string     `json:"transferDirection,omitempty"`
		Type              string     `json:"type"`
		Status            string     `json:"status"`
		Amount            float64    `json:"amount"`
		InstallmentNumber *int       `json:"installmentNumber,omitempty"`
		AccountID         *uuid.UUID `json:"accountId,omitempty"`
		AccountName       string     `json:"accountName,omitempty"`
		Source            string     `json:"source"`
	}
	items := make([]item, len(txs))
	for i, t := range txs {
		source := "MANUAL"
		if t.FromOpenFinance {
			source = "OPEN_FINANCE"
		}
		items[i] = item{t.ID, t.Date.Format(time.DateOnly), t.Description, t.Category, domain.Category(t.Category).Label(),
			t.PaymentMethod, domain.PaymentMethod(t.PaymentMethod).Label(), t.Kind, t.TransferDirection, t.Type, t.Status,
			t.Amount, t.InstallmentNumber, t.AccountID, t.AccountName, source}
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items, "total": total, "page": page, "limit": f.Limit})
}

func (a *api) registerTransactions(rt routes) {
	rt.mux.Handle("GET /api/transactions", rt.protected(a.transactions))
	rt.mux.Handle("GET /api/transactions/groups", rt.protected(a.transactionGroups))
}
