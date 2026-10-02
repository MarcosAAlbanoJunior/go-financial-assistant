package httpserver

import (
	"context"
	"errors"
	"mime"
	"net/http"
	"time"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/usecase"
	"github.com/google/uuid"
)

// syncTimeout dá folga para vários itens do Pluggy; a sincronização não é cortada se a aba for fechada no meio.
const syncTimeout = 5 * time.Minute

type balanceAccountJSON struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	Last4        string   `json:"last4"`
	Balance      float64  `json:"balance"`
	AutoInvested *float64 `json:"autoInvested"`
}

type balanceCardJSON struct {
	ID             string        `json:"id"`
	Name           string        `json:"name"`
	Brand          string        `json:"brand"`
	Last4          string        `json:"last4"`
	Invoice        float64       `json:"invoice"`
	Limit          *float64      `json:"limit"`
	Available      *float64      `json:"available"`
	Used           *float64      `json:"used"`
	UsedRatio      *float64      `json:"usedRatio"`
	UsageLevel     usecase.Level `json:"usageLevel"`
	CloseDate      *string       `json:"closeDate"`
	DueDate        *string       `json:"dueDate"`
	DaysToDue      *int          `json:"daysToDue"`
	DueLevel       usecase.Level `json:"dueLevel"`
	MinimumPayment *float64      `json:"minimumPayment"`
}

type institutionJSON struct {
	ID        string               `json:"id"`
	Name      string               `json:"name"`
	Color     *string              `json:"color"`
	Logo      *string              `json:"logo"`
	UpdatedAt time.Time            `json:"updatedAt"`
	Stale     bool                 `json:"stale"`
	Total     *float64             `json:"total"`
	Share     float64              `json:"shareOfTotal"`
	Accounts  []balanceAccountJSON `json:"accounts"`
	Cards     []balanceCardJSON    `json:"cards"`
}

func dayString(t *time.Time) *string {
	if t == nil {
		return nil
	}
	s := t.UTC().Format("2006-01-02")
	return &s
}

// balances devolve os saldos por banco. O item_id do Pluggy nunca sai: o id da instituição é opaco.
func (a *api) balances(w http.ResponseWriter, r *http.Request) {
	view, err := a.insights.Balances(r.Context(), a.now())
	if err != nil {
		a.fail(w, "saldos", err)
		return
	}
	institutions := make([]institutionJSON, len(view.Institutions))
	for i, in := range view.Institutions {
		j := institutionJSON{
			ID: in.ID, Name: in.Name, UpdatedAt: in.UpdatedAt, Stale: in.Stale, Share: in.Share,
			Accounts: make([]balanceAccountJSON, len(in.Accounts)), Cards: make([]balanceCardJSON, len(in.Cards)),
		}
		if in.Color != "" {
			c := "#" + in.Color
			j.Color = &c
		}
		if in.HasLogo {
			l := "/api/institutions/" + in.ID + "/logo"
			j.Logo = &l
		}
		if in.HasBank {
			t := in.Total
			j.Total = &t
		}
		for n, acc := range in.Accounts {
			j.Accounts[n] = balanceAccountJSON(acc)
		}
		for n, c := range in.Cards {
			j.Cards[n] = balanceCardJSON{
				ID: c.ID, Name: c.Name, Brand: c.Brand, Last4: c.Last4, Invoice: c.Invoice, Limit: c.Limit, Available: c.Available,
				Used: c.Used, UsedRatio: c.UsedRatio, UsageLevel: c.UsageLevel, CloseDate: dayString(c.CloseDate),
				DueDate: dayString(c.DueDate), DaysToDue: c.DaysToDue, DueLevel: c.DueLevel, MinimumPayment: c.MinimumPayment,
			}
		}
		institutions[i] = j
	}
	var asOf *time.Time
	if !view.AsOf.IsZero() {
		asOf = &view.AsOf
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"asOf": asOf, "totalInAccount": view.TotalInAccount, "openInvoices": view.OpenInvoices, "institutions": institutions,
	})
}

// institutionLogo serve o logo em cache. O tipo vem de uma lista fechada (checada no banco e na busca); mesmo assim o
// SVG vai com CSP restritivo e nosniff, para que não execute nada nem se for aberto direto.
func (a *api) institutionLogo(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusNotFound, "logo não encontrado")
		return
	}
	data, mimeType, found, err := a.reader.InstitutionLogo(r.Context(), id)
	if err != nil {
		a.fail(w, "logo da instituição", err)
		return
	}
	if !found {
		writeError(w, http.StatusNotFound, "logo não encontrado")
		return
	}
	h := w.Header()
	h.Set("Content-Type", mimeType)
	h.Set("X-Content-Type-Options", "nosniff")
	h.Set("Content-Security-Policy", "default-src 'none'; sandbox")
	h.Set("Cache-Control", "private, max-age=86400")
	w.Write(data) //nolint:errcheck
}

// syncNow dispara a mesma sincronização do /sync do chat. É a única escrita sem corpo da API: exige JSON e mesma origem.
func (a *api) syncNow(w http.ResponseWriter, r *http.Request) {
	if mt, _, _ := mime.ParseMediaType(r.Header.Get("Content-Type")); mt != "application/json" || !sameOrigin(r) {
		writeError(w, http.StatusForbidden, "requisição não permitida")
		return
	}
	if a.syncer == nil {
		writeError(w, http.StatusServiceUnavailable, "o Open Finance não está configurado")
		return
	}
	ctx, cancel := context.WithTimeout(context.WithoutCancel(r.Context()), syncTimeout)
	defer cancel()

	result, err := a.syncer.Sync(ctx)
	if errors.Is(err, usecase.ErrSyncInProgress) {
		writeError(w, http.StatusConflict, "já existe uma sincronização em andamento")
		return
	}
	if err != nil {
		// Falhas parciais (um item) já foram registradas; as contas que deram certo foram atualizadas.
		a.logger.Error("erro na sincronização pelo dashboard", "error", err)
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"inserted": result.Inserted, "reconciled": result.Reconciled, "existing": result.Existing, "positions": result.Positions,
		"partial": err != nil,
	})
}
