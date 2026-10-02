package httpserver

import (
	"net/http"
	"time"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain"
)

const (
	maxSeriesMonths = 60
	defaultPageSize = 50
	maxPageSize     = 100
	maxSearchLen    = 100
	maxLoginBody    = 1 << 10
	maxRuleBody     = 1 << 10
)

var (
	categories     = []domain.Category{domain.CategoryFood, domain.CategoryTransport, domain.CategoryHealth, domain.CategoryEntertainment, domain.CategoryShopping, domain.CategoryMarket, domain.CategoryInvestment, domain.CategorySalary, domain.CategoryHousing, domain.CategoryBills, domain.CategoryEducation, domain.CategoryPeople, domain.CategoryOther}
	paymentMethods = []domain.PaymentMethod{domain.PaymentMethodCash, domain.PaymentMethodCreditCard, domain.PaymentMethodDebitCard, domain.PaymentMethodPix, domain.PaymentMethodOther}
	kinds          = []domain.PurchaseKind{domain.KindExpense, domain.KindIncome, domain.KindTransfer}
)

// monthParam lê um mês "YYYY-MM" e devolve o seu primeiro dia (UTC). Se obrigatório e
// ausente, responde 400; se opcional e ausente, devolve a data zero.
func (a *api) monthParam(w http.ResponseWriter, r *http.Request, name string, required bool) (time.Time, bool) {
	raw := r.URL.Query().Get(name)
	if raw == "" && !required {
		return time.Time{}, true
	}
	t, err := time.Parse("2006-01", raw)
	if err != nil {
		writeError(w, http.StatusBadRequest, name+" deve estar no formato AAAA-MM")
		return time.Time{}, false
	}
	return t, true
}

// rangeParams lê from e to (padrão: últimos 12 meses até o mês atual) e limita a janela.
func (a *api) rangeParams(w http.ResponseWriter, r *http.Request) (from, to time.Time, ok bool) {
	now := a.now().UTC()
	current := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	to, from = current, current.AddDate(0, -11, 0)

	q := r.URL.Query()
	if q.Get("to") != "" {
		if to, ok = a.monthParam(w, r, "to", true); !ok {
			return
		}
	}
	if q.Get("from") != "" {
		if from, ok = a.monthParam(w, r, "from", true); !ok {
			return
		}
	} else if q.Get("to") != "" {
		from = to.AddDate(0, -11, 0)
	}
	months := (to.Year()-from.Year())*12 + int(to.Month()-from.Month()) + 1
	if months < 1 || months > maxSeriesMonths {
		writeError(w, http.StatusBadRequest, "o período deve ter de 1 a 60 meses")
		return time.Time{}, time.Time{}, false
	}
	return from, to, true
}

func enumParam[T ~string](w http.ResponseWriter, value, name string, allowed []T) (string, bool) {
	if value == "" {
		return "", true
	}
	for _, v := range allowed {
		if string(v) == value {
			return value, true
		}
	}
	writeError(w, http.StatusBadRequest, name+" inválido")
	return "", false
}

func formatMonth(t time.Time) string { return t.Format("2006-01") }
