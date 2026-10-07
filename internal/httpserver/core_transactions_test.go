package httpserver

import (
	"encoding/json"
	"strconv"
	"strings"
	"testing"

	"hohoho/internal/config"
	"hohoho/internal/store"
)

func TestCoreManualTransactionAPI(t *testing.T) {
	handler := coreTestHandler(t, config.Runtime{})
	coreRequest(t, handler, "GET", "/api/v1/transactions", "", 200)
	objectResponse := coreRequest(t, handler, "POST", "/api/v1/financial-objects", `{"objectType":"expense","name":"Boodschappen"}`, 201)
	var object store.FinancialObject
	if err := json.Unmarshal(objectResponse.Body.Bytes(), &object); err != nil {
		t.Fatal(err)
	}
	body := `{"transactionDate":"2026-10-01","amountCents":-8342,"description":"ALDI","counterparty":"ALDI","allocations":[{"financialObjectId":` + strconv.FormatInt(object.ID, 10) + `,"amountCents":-8342}]}`
	response := coreRequest(t, handler, "POST", "/api/v1/transactions", body, 201)
	var item store.CoreTransaction
	if err := json.Unmarshal(response.Body.Bytes(), &item); err != nil || !item.FullyAllocated || len(item.Allocations) != 1 || item.Allocations[0].Name != "Boodschappen" {
		t.Fatalf("allocation response: %s, %v", response.Body.String(), err)
	}
	path := "/api/v1/transactions/" + strconv.FormatInt(item.ID, 10)
	for _, body := range []string{
		`{"transactionDate":"2026-10-01"}`,
		`{"transactionDate":"2026-10-01","amountCents":0}`,
		`{"transactionDate":"2026-02-30","amountCents":100}`,
		`{"transactionDate":"2026-10-01","amountCents":1.5}`,
		`{"transactionDate":"2026-10-01","amountCents":100,"financialObjectId":1}`,
		`{"transactionDate":"2026-10-01","amountCents":100,"allocations":[{"financialObjectId":1,"amountCents":-100}]}`,
	} {
		coreRequest(t, handler, "PUT", path, body, 400)
	}
	coreRequest(t, handler, "GET", path, "", 200)
	for _, query := range []string{"direction=invalid", "visibility=invalid", "from=2026-02-30", "to=2026-13-01", "sort=invalid", "order=sideways", "limit=0", "limit=101", "offset=-1", "limit=abc"} {
		coreRequest(t, handler, "GET", "/api/v1/transactions?"+query, "", 400)
	}
	coreRequest(t, handler, "DELETE", "/api/v1/financial-objects/"+strconv.FormatInt(object.ID, 10), "", 409)
	coreRequest(t, handler, "PUT", path, `{"transactionDate":"2026-10-02","amountCents":10000,"description":"Ontvangst","allocations":[]}`, 200)
	response = coreRequest(t, handler, "GET", "/api/v1/transactions?direction=income&limit=1", "", 200)
	var page store.CoreTransactionPage
	if err := json.Unmarshal(response.Body.Bytes(), &page); err != nil || page.Total != 1 || page.IncomeCents != 10000 || page.BalanceCents != 10000 || len(page.Items[0].Allocations) != 0 {
		t.Fatalf("income summary: %s, %v", response.Body.String(), err)
	}
	coreRequest(t, handler, "DELETE", path, "", 204)
	response = coreRequest(t, handler, "GET", path, "", 200)
	if err := json.Unmarshal(response.Body.Bytes(), &item); err != nil || !item.Hidden {
		t.Fatalf("transaction must be hidden, not deleted: %s, %v", response.Body.String(), err)
	}
	response = coreRequest(t, handler, "GET", "/api/v1/transactions", "", 200)
	if !strings.Contains(response.Body.String(), `"items":[]`) {
		t.Fatalf("empty list must be an array: %s", response.Body.String())
	}
	response = coreRequest(t, handler, "GET", "/api/v1/transactions?visibility=hidden", "", 200)
	if !strings.Contains(response.Body.String(), `"hidden":true`) {
		t.Fatalf("hidden list must contain hidden transaction: %s", response.Body.String())
	}
	coreRequest(t, handler, "POST", path+"/unhide", "", 204)
	response = coreRequest(t, handler, "GET", "/api/v1/transactions", "", 200)
	if !strings.Contains(response.Body.String(), `"hidden":false`) {
		t.Fatalf("transaction must be visible again: %s", response.Body.String())
	}
	coreRequest(t, handler, "DELETE", "/api/v1/financial-objects/"+strconv.FormatInt(object.ID, 10), "", 204)
}

func TestCoreTransactionFilters(t *testing.T) {
	handler := coreTestHandler(t, config.Runtime{})
	objectResponse := coreRequest(t, handler, "POST", "/api/v1/financial-objects", `{"objectType":"expense","name":"Boodschappen"}`, 201)
	var object store.FinancialObject
	if err := json.Unmarshal(objectResponse.Body.Bytes(), &object); err != nil {
		t.Fatal(err)
	}
	coreRequest(t, handler, "POST", "/api/v1/transactions", `{"transactionDate":"2026-01-10","amountCents":-1000,"description":"Supermarkt","counterparty":"ALDI","allocations":[]}`, 201)
	coreRequest(t, handler, "POST", "/api/v1/transactions", `{"transactionDate":"2026-02-10","amountCents":-2000,"description":"Zorgpremie","counterparty":"FBTO","allocations":[{"financialObjectId":`+strconv.FormatInt(object.ID, 10)+`,"amountCents":-2000}]}`, 201)
	coreRequest(t, handler, "POST", "/api/v1/transactions", `{"transactionDate":"2026-03-10","amountCents":3000,"description":"Terugbetaling","counterparty":"FBTO","allocations":[]}`, 201)
	response := coreRequest(t, handler, "GET", "/api/v1/transactions?from=2026-02-01&to=2026-02-28", "", 200)
	if text := response.Body.String(); !strings.Contains(text, `"total":1`) || !strings.Contains(text, "Zorgpremie") || strings.Contains(text, "Supermarkt") {
		t.Fatalf("date filter failed: %s", text)
	}
	response = coreRequest(t, handler, "GET", "/api/v1/transactions?search=fbto", "", 200)
	if text := response.Body.String(); !strings.Contains(text, `"total":2`) || !strings.Contains(text, "Terugbetaling") || strings.Contains(text, "ALDI") {
		t.Fatalf("search filter failed: %s", text)
	}
	response = coreRequest(t, handler, "GET", "/api/v1/transactions?sort=amount&order=asc", "", 200)
	var page store.CoreTransactionPage
	if err := json.Unmarshal(response.Body.Bytes(), &page); err != nil || len(page.Items) != 3 || page.Items[0].AmountCents != -2000 || page.Items[2].AmountCents != 3000 {
		t.Fatalf("amount sort failed: %s, %v", response.Body.String(), err)
	}
	response = coreRequest(t, handler, "GET", "/api/v1/transactions?sort=description&order=asc", "", 200)
	if err := json.Unmarshal(response.Body.Bytes(), &page); err != nil || len(page.Items) != 3 || page.Items[0].Description != "Supermarkt" || page.Items[2].Description != "Zorgpremie" {
		t.Fatalf("description sort failed: %s, %v", response.Body.String(), err)
	}
	response = coreRequest(t, handler, "GET", "/api/v1/transactions?financialObjectId="+strconv.FormatInt(object.ID, 10), "", 200)
	if err := json.Unmarshal(response.Body.Bytes(), &page); err != nil || page.Total != 1 || page.Items[0].Description != "Zorgpremie" || len(page.Items[0].Allocations) != 1 {
		t.Fatalf("financial object filter failed: %s, %v", response.Body.String(), err)
	}
}

func TestCoreTransactionAppliesCategorizationToMatchingCounterparty(t *testing.T) {
	handler := coreTestHandler(t, config.Runtime{})
	objectResponse := coreRequest(t, handler, "POST", "/api/v1/financial-objects", `{"objectType":"expense","name":"Zorgverzekering"}`, 201)
	var object store.FinancialObject
	if err := json.Unmarshal(objectResponse.Body.Bytes(), &object); err != nil {
		t.Fatal(err)
	}
	first := coreRequest(t, handler, "POST", "/api/v1/transactions", `{"transactionDate":"2026-01-01","amountCents":-1000,"description":"Premie","counterparty":"FBTO","allocations":[]}`, 201)
	second := coreRequest(t, handler, "POST", "/api/v1/transactions", `{"transactionDate":"2026-02-01","amountCents":-1200,"description":"Premie","counterparty":"FBTO","allocations":[]}`, 201)
	var firstItem, secondItem store.CoreTransaction
	if err := json.Unmarshal(first.Body.Bytes(), &firstItem); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(second.Body.Bytes(), &secondItem); err != nil {
		t.Fatal(err)
	}
	body := `{"transactionDate":"2026-01-01","amountCents":-1000,"description":"Premie","counterparty":"FBTO","applyToMatching":true,"allocations":[{"financialObjectId":` + strconv.FormatInt(object.ID, 10) + `,"amountCents":-1000}]}`
	coreRequest(t, handler, "PUT", "/api/v1/transactions/"+strconv.FormatInt(firstItem.ID, 10), body, 200)
	response := coreRequest(t, handler, "GET", "/api/v1/transactions/"+strconv.FormatInt(secondItem.ID, 10), "", 200)
	if err := json.Unmarshal(response.Body.Bytes(), &secondItem); err != nil || len(secondItem.Allocations) != 1 || secondItem.Allocations[0].AmountCents == nil || *secondItem.Allocations[0].AmountCents != -1200 {
		t.Fatalf("matching counterparty was not categorized with target amount: %s, %v", response.Body.String(), err)
	}
}
