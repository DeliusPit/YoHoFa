package httpserver

import (
	"encoding/json"
	"net/http"
	"strconv"

	"hohoho/internal/store"
)

func registerLiabilityRoutes(mux *http.ServeMux, deps Dependencies) {
	mux.HandleFunc("GET /api/v1/liabilities", func(w http.ResponseWriter, r *http.Request) {
		items, err := deps.Liabilities.Liabilities(r.Context(), store.LiabilityFilter{
			LiabilityType: r.URL.Query().Get("type"), Status: r.URL.Query().Get("status"),
		})
		if err != nil {
			deps.Logger.Error("list liabilities", "error", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal_error"})
			return
		}
		writeJSON(w, http.StatusOK, items)
	})

	mux.HandleFunc("POST /api/v1/liabilities", func(w http.ResponseWriter, r *http.Request) {
		var input store.SaveLiability
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_json"})
			return
		}
		item, err := deps.Liabilities.CreateLiability(r.Context(), input)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusCreated, item)
	})

	mux.HandleFunc("GET /api/v1/liabilities/{id}", func(w http.ResponseWriter, r *http.Request) {
		id, err := pathID(r)
		if err != nil || id <= 0 {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_id"})
			return
		}
		item, err := deps.Liabilities.Liability(r.Context(), id)
		if err != nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "liability_not_found"})
			return
		}
		writeJSON(w, http.StatusOK, item)
	})

	mux.HandleFunc("PATCH /api/v1/liabilities/{id}", func(w http.ResponseWriter, r *http.Request) {
		id, err := pathID(r)
		if err != nil || id <= 0 {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_id"})
			return
		}
		var input store.SaveLiability
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_json"})
			return
		}
		item, err := deps.Liabilities.UpdateLiability(r.Context(), id, input)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, item)
	})

	mux.HandleFunc("DELETE /api/v1/liabilities/{id}", func(w http.ResponseWriter, r *http.Request) {
		id, err := pathID(r)
		if err != nil || id <= 0 {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_id"})
			return
		}
		if err := deps.Liabilities.DeleteLiability(r.Context(), id); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})

	mux.HandleFunc("GET /api/v1/liabilities/{id}/transactions", func(w http.ResponseWriter, r *http.Request) {
		id, err := pathID(r)
		if err != nil || id <= 0 {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_id"})
			return
		}
		items, err := deps.Relations.LiabilityTransactions(r.Context(), id)
		if err != nil {
			deps.Logger.Error("list liability transactions", "error", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal_error"})
			return
		}
		writeJSON(w, http.StatusOK, items)
	})

	mux.HandleFunc("POST /api/v1/liabilities/{id}/transactions", func(w http.ResponseWriter, r *http.Request) {
		id, err := pathID(r)
		if err != nil || id <= 0 {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_id"})
			return
		}
		var input struct {
			TransactionID int64 `json:"transactionId"`
		}
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil || input.TransactionID <= 0 {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_transaction"})
			return
		}
		if err := deps.Relations.LinkLiabilityTransaction(r.Context(), id, input.TransactionID); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})

	mux.HandleFunc("DELETE /api/v1/liabilities/{id}/transactions/{transactionId}", func(w http.ResponseWriter, r *http.Request) {
		id, err := pathID(r)
		if err != nil || id <= 0 {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_id"})
			return
		}
		transactionID, err := strconv.ParseInt(r.PathValue("transactionId"), 10, 64)
		if err != nil || transactionID <= 0 {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_transaction_id"})
			return
		}
		if err := deps.Relations.UnlinkLiabilityTransaction(r.Context(), id, transactionID); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})

	mux.HandleFunc("GET /api/v1/liabilities/{id}/relations", func(w http.ResponseWriter, r *http.Request) {
		id, err := pathID(r)
		if err != nil || id <= 0 {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_id"})
			return
		}
		items, err := deps.Relations.RelationsForObject(r.Context(), store.ObjectTypeLiability, id)
		if err != nil {
			deps.Logger.Error("list liability relations", "error", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal_error"})
			return
		}
		writeJSON(w, http.StatusOK, items)
	})

	mux.HandleFunc("POST /api/v1/liabilities/{id}/relations", func(w http.ResponseWriter, r *http.Request) {
		id, err := pathID(r)
		if err != nil || id <= 0 {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_id"})
			return
		}
		var input struct {
			TargetType   string `json:"targetType"`
			TargetID     int64  `json:"targetId"`
			RelationType string `json:"relationType"`
			ValidFrom    string `json:"validFrom"`
			ValidTo      string `json:"validTo"`
			Notes        string `json:"notes"`
		}
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil || input.TargetID <= 0 {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_relation"})
			return
		}
		item, err := deps.Relations.CreateEntityRelation(r.Context(), store.SaveEntityRelation{
			SourceType: store.ObjectTypeLiability, SourceID: id,
			TargetType: input.TargetType, TargetID: input.TargetID, RelationType: input.RelationType,
			ValidFrom: input.ValidFrom, ValidTo: input.ValidTo, Notes: input.Notes,
		})
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusCreated, item)
	})

	mux.HandleFunc("DELETE /api/v1/liabilities/{id}/relations/{relationId}", func(w http.ResponseWriter, r *http.Request) {
		id, err := pathID(r)
		if err != nil || id <= 0 {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_id"})
			return
		}
		relationID, err := strconv.ParseInt(r.PathValue("relationId"), 10, 64)
		if err != nil || relationID <= 0 {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_relation_id"})
			return
		}
		if err := deps.Relations.DeleteEntityRelation(r.Context(), relationID, store.ObjectTypeLiability, id); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
}

func registerLiabilityDocumentRoutes(mux *http.ServeMux, deps Dependencies) {
	mux.HandleFunc("GET /api/v1/liabilities/{id}/documents", func(w http.ResponseWriter, r *http.Request) {
		id, err := pathID(r)
		if err != nil || id <= 0 {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_id"})
			return
		}
		items, err := deps.Documents.DocumentsForObject(r.Context(), store.ObjectTypeLiability, id)
		if err != nil {
			deps.Logger.Error("list liability documents", "error", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal_error"})
			return
		}
		writeJSON(w, http.StatusOK, items)
	})

	mux.HandleFunc("POST /api/v1/liabilities/{id}/documents", func(w http.ResponseWriter, r *http.Request) {
		id, err := pathID(r)
		if err != nil || id <= 0 {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_id"})
			return
		}
		var input struct {
			DocumentID int64 `json:"documentId"`
		}
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil || input.DocumentID <= 0 {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_document"})
			return
		}
		if err := deps.Documents.LinkDocument(r.Context(), input.DocumentID, store.ObjectTypeLiability, id); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})

	mux.HandleFunc("DELETE /api/v1/liabilities/{id}/documents/{documentId}", func(w http.ResponseWriter, r *http.Request) {
		id, err := pathID(r)
		if err != nil || id <= 0 {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_id"})
			return
		}
		documentID, err := strconv.ParseInt(r.PathValue("documentId"), 10, 64)
		if err != nil || documentID <= 0 {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_document_id"})
			return
		}
		if err := deps.Documents.UnlinkDocument(r.Context(), documentID, store.ObjectTypeLiability, id); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
}
