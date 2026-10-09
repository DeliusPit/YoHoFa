package httpserver

import (
	"net/http"
	"strconv"

	"hohoho/internal/store"
)

func registerCoreTransactionRoutes(mux *http.ServeMux, deps CoreDependencies) {
	mux.HandleFunc("GET /api/v1/transactions", func(w http.ResponseWriter, r *http.Request) {
		filter := store.CoreTransactionFilter{Direction: r.URL.Query().Get("direction"), Visibility: r.URL.Query().Get("visibility"), From: r.URL.Query().Get("from"), To: r.URL.Query().Get("to"), Search: r.URL.Query().Get("search"), Sort: r.URL.Query().Get("sort"), Order: r.URL.Query().Get("order"), Status: r.URL.Query().Get("status")}
		for key, destination := range map[string]*int{"limit": &filter.Limit, "offset": &filter.Offset} {
			if value := r.URL.Query().Get(key); value != "" {
				parsed, err := strconv.Atoi(value)
				if err != nil || (key == "limit" && parsed <= 0) || (key == "offset" && parsed < 0) {
					coreRespond(w, r, deps, 0, nil, store.ErrCoreInvalid)
					return
				}
				*destination = parsed
			}
		}
		if value := r.URL.Query().Get("financialObjectId"); value != "" {
			parsed, err := strconv.ParseInt(value, 10, 64)
			if err != nil || parsed <= 0 {
				coreRespond(w, r, deps, 0, nil, store.ErrCoreInvalid)
				return
			}
			filter.FinancialObjectID = parsed
		}
		items, err := deps.Store.CoreTransactions(r.Context(), filter)
		coreRespond(w, r, deps, http.StatusOK, items, err)
	})
	mux.HandleFunc("POST /api/v1/transactions/classify-ai", func(w http.ResponseWriter, r *http.Request) {
		pageI, err := deps.Store.CoreTransactions(r.Context(), store.CoreTransactionFilter{Status: "I", Limit: 100, Visibility: "visible"})
		if err != nil {
			coreRespond(w, r, deps, 0, nil, err)
			return
		}
		pageF, err := deps.Store.CoreTransactions(r.Context(), store.CoreTransactionFilter{Status: "F", Limit: 100, Visibility: "visible"})
		if err != nil {
			coreRespond(w, r, deps, 0, nil, err)
			return
		}
		transactions := append(pageI.Items, pageF.Items...)
		if len(transactions) == 0 {
			coreRespond(w, r, deps, http.StatusOK, map[string]int{"classified": 0, "failed": 0}, nil)
			return
		}
		allocations, failed, err := classifyTransactionsWithAI(r.Context(), deps, transactions)
		if err != nil {
			writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
			return
		}
		if err := deps.Store.ApplyAITransactionClassifications(r.Context(), allocations, failed); err != nil {
			coreRespond(w, r, deps, 0, nil, err)
			return
		}
		coreRespond(w, r, deps, http.StatusOK, map[string]int{"classified": len(allocations), "failed": len(failed)}, nil)
	})
	mux.HandleFunc("POST /api/v1/transactions/bulk-classify", func(w http.ResponseWriter, r *http.Request) {
		var input store.BulkCoreTransactionClassification
		if !coreDecode(w, r, &input) {
			return
		}
		err := deps.Store.BulkClassifyCoreTransactions(r.Context(), input)
		coreRespond(w, r, deps, http.StatusNoContent, nil, err)
	})
	mux.HandleFunc("POST /api/v1/transactions", func(w http.ResponseWriter, r *http.Request) {
		var input store.SaveCoreTransaction
		if !coreDecode(w, r, &input) {
			return
		}
		item, err := deps.Store.CreateCoreTransaction(r.Context(), input)
		coreRespond(w, r, deps, http.StatusCreated, item, err)
	})
	mux.HandleFunc("GET /api/v1/transactions/{id}", coreObjectHandler(func(w http.ResponseWriter, r *http.Request, id int64) {
		item, err := deps.Store.CoreTransaction(r.Context(), id)
		coreRespond(w, r, deps, http.StatusOK, item, err)
	}))
	mux.HandleFunc("PUT /api/v1/transactions/{id}", coreObjectHandler(func(w http.ResponseWriter, r *http.Request, id int64) {
		var input store.SaveCoreTransaction
		if !coreDecode(w, r, &input) {
			return
		}
		item, err := deps.Store.UpdateCoreTransaction(r.Context(), id, input)
		coreRespond(w, r, deps, http.StatusOK, item, err)
	}))
	mux.HandleFunc("DELETE /api/v1/transactions/{id}", coreObjectHandler(func(w http.ResponseWriter, r *http.Request, id int64) {
		err := deps.Store.DeleteCoreTransaction(r.Context(), id)
		coreRespond(w, r, deps, http.StatusNoContent, nil, err)
	}))
	mux.HandleFunc("POST /api/v1/transactions/{id}/unhide", coreObjectHandler(func(w http.ResponseWriter, r *http.Request, id int64) {
		err := deps.Store.SetCoreTransactionHidden(r.Context(), id, false)
		coreRespond(w, r, deps, http.StatusNoContent, nil, err)
	}))
}
