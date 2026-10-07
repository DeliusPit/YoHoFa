package httpserver

import (
	"encoding/json"
	"testing"

	"hohoho/internal/config"
	"hohoho/internal/store"
)

func TestCoreDashboardAPI(t *testing.T) {
	handler := coreTestHandler(t, config.Runtime{})
	coreRequest(t, handler, "POST", "/api/v1/transactions", `{"transactionDate":"2026-10-01","amountCents":-8342}`, 201)
	coreRequest(t, handler, "POST", "/api/v1/transactions", `{"transactionDate":"2026-11-01","amountCents":10000}`, 201)
	response := coreRequest(t, handler, "GET", "/api/v1/dashboard?year=2026&month=10", "", 200)
	var item store.CoreDashboard
	if err := json.Unmarshal(response.Body.Bytes(), &item); err != nil || item.Year != 2026 || item.Month != 10 || item.Transactions.Count != 1 || item.Transactions.ExpenseCents != -8342 {
		t.Fatalf("dashboard response: %s, %v", response.Body.String(), err)
	}
	response = coreRequest(t, handler, "GET", "/api/v1/dashboard?year=2026", "", 200)
	if err := json.Unmarshal(response.Body.Bytes(), &item); err != nil || item.Transactions.Count != 2 || item.Transactions.BalanceCents != 1658 {
		t.Fatalf("whole-year dashboard: %s, %v", response.Body.String(), err)
	}
	for _, query := range []string{"year=0", "year=-1", "year=10000", "year=abc", "month=-1", "month=13", "month=1.5"} {
		coreRequest(t, handler, "GET", "/api/v1/dashboard?"+query, "", 400)
	}
}
