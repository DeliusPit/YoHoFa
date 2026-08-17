package httpserver

import (
	"encoding/json"
	"net/http"
	"strconv"

	"geldzaken/internal/store"
)

func registerAssetRoutes(mux *http.ServeMux, deps Dependencies) {
	mux.HandleFunc("GET /api/v1/assets", func(w http.ResponseWriter, r *http.Request) {
		assets, err := deps.Assets.Assets(r.Context(), store.AssetFilter{AssetType: r.URL.Query().Get("type"), Status: r.URL.Query().Get("status")})
		if err != nil {
			deps.Logger.Error("list assets", "error", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal_error"})
			return
		}
		writeJSON(w, http.StatusOK, assets)
	})

	mux.HandleFunc("POST /api/v1/assets", func(w http.ResponseWriter, r *http.Request) {
		var input store.SaveAsset
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_json"})
			return
		}
		asset, err := deps.Assets.CreateAsset(r.Context(), input)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusCreated, asset)
	})

	mux.HandleFunc("GET /api/v1/assets/{id}", func(w http.ResponseWriter, r *http.Request) {
		id, err := pathID(r)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_id"})
			return
		}
		asset, err := deps.Assets.Asset(r.Context(), id)
		if err != nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "asset_not_found"})
			return
		}
		writeJSON(w, http.StatusOK, asset)
	})

	mux.HandleFunc("PATCH /api/v1/assets/{id}", func(w http.ResponseWriter, r *http.Request) {
		id, err := pathID(r)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_id"})
			return
		}
		var input store.SaveAsset
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_json"})
			return
		}
		asset, err := deps.Assets.UpdateAsset(r.Context(), id, input)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, asset)
	})

	mux.HandleFunc("DELETE /api/v1/assets/{id}", func(w http.ResponseWriter, r *http.Request) {
		id, err := pathID(r)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_id"})
			return
		}
		if err := deps.Assets.DeleteAsset(r.Context(), id); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})

	mux.HandleFunc("GET /api/v1/assets/{id}/valuations", func(w http.ResponseWriter, r *http.Request) {
		assetID, err := pathID(r)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_id"})
			return
		}
		valuations, err := deps.Assets.AssetValuations(r.Context(), assetID)
		if err != nil {
			deps.Logger.Error("list asset valuations", "error", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal_error"})
			return
		}
		writeJSON(w, http.StatusOK, valuations)
	})

	mux.HandleFunc("POST /api/v1/assets/{id}/valuations", func(w http.ResponseWriter, r *http.Request) {
		assetID, err := pathID(r)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_id"})
			return
		}
		var input store.SaveAssetValuation
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_json"})
			return
		}
		valuation, err := deps.Assets.CreateAssetValuation(r.Context(), assetID, input)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusCreated, valuation)
	})

	mux.HandleFunc("PATCH /api/v1/assets/{id}/valuations/{valuationId}", func(w http.ResponseWriter, r *http.Request) {
		assetID, err := pathID(r)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_id"})
			return
		}
		valuationID, err := strconv.ParseInt(r.PathValue("valuationId"), 10, 64)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_valuation_id"})
			return
		}
		var input store.SaveAssetValuation
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_json"})
			return
		}
		valuation, err := deps.Assets.UpdateAssetValuation(r.Context(), assetID, valuationID, input)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, valuation)
	})

	mux.HandleFunc("DELETE /api/v1/assets/{id}/valuations/{valuationId}", func(w http.ResponseWriter, r *http.Request) {
		assetID, err := pathID(r)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_id"})
			return
		}
		valuationID, err := strconv.ParseInt(r.PathValue("valuationId"), 10, 64)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_valuation_id"})
			return
		}
		if err := deps.Assets.DeleteAssetValuation(r.Context(), assetID, valuationID); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})

	mux.HandleFunc("GET /api/v1/assets/{id}/transactions", func(w http.ResponseWriter, r *http.Request) {
		assetID, err := pathID(r)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_id"})
			return
		}
		transactions, err := deps.Relations.AssetTransactions(r.Context(), assetID)
		if err != nil {
			deps.Logger.Error("list asset transactions", "error", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal_error"})
			return
		}
		writeJSON(w, http.StatusOK, transactions)
	})

	mux.HandleFunc("POST /api/v1/assets/{id}/transactions", func(w http.ResponseWriter, r *http.Request) {
		assetID, err := pathID(r)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_id"})
			return
		}
		var input struct {
			TransactionID int64 `json:"transactionId"`
		}
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil || input.TransactionID == 0 {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_transaction"})
			return
		}
		if err := deps.Relations.LinkAssetTransaction(r.Context(), assetID, input.TransactionID); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})

	mux.HandleFunc("DELETE /api/v1/assets/{id}/transactions/{transactionId}", func(w http.ResponseWriter, r *http.Request) {
		assetID, err := pathID(r)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_id"})
			return
		}
		transactionID, err := strconv.ParseInt(r.PathValue("transactionId"), 10, 64)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_transaction_id"})
			return
		}
		if err := deps.Relations.UnlinkAssetTransaction(r.Context(), assetID, transactionID); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})

	mux.HandleFunc("GET /api/v1/assets/{id}/relations", func(w http.ResponseWriter, r *http.Request) {
		assetID, err := pathID(r)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_id"})
			return
		}
		relations, err := deps.Relations.RelationsForObject(r.Context(), store.ObjectTypeAsset, assetID)
		if err != nil {
			deps.Logger.Error("list asset relations", "error", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal_error"})
			return
		}
		writeJSON(w, http.StatusOK, relations)
	})

	mux.HandleFunc("POST /api/v1/assets/{id}/relations", func(w http.ResponseWriter, r *http.Request) {
		assetID, err := pathID(r)
		if err != nil {
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
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil || input.TargetID == 0 {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_relation"})
			return
		}
		relationInput := store.SaveEntityRelation{
			SourceType: store.ObjectTypeAsset, SourceID: assetID,
			TargetType: input.TargetType, TargetID: input.TargetID,
			RelationType: input.RelationType, ValidFrom: input.ValidFrom, ValidTo: input.ValidTo, Notes: input.Notes,
		}
		relation, err := deps.Relations.CreateEntityRelation(r.Context(), relationInput)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusCreated, relation)
	})

	mux.HandleFunc("DELETE /api/v1/assets/{id}/relations/{relationId}", func(w http.ResponseWriter, r *http.Request) {
		assetID, err := pathID(r)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_id"})
			return
		}
		relationID, err := strconv.ParseInt(r.PathValue("relationId"), 10, 64)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_relation_id"})
			return
		}
		if err := deps.Relations.DeleteEntityRelation(r.Context(), relationID, store.ObjectTypeAsset, assetID); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})

}

func registerAssetDocumentRoutes(mux *http.ServeMux, deps Dependencies) {
	mux.HandleFunc("GET /api/v1/assets/{id}/documents", func(w http.ResponseWriter, r *http.Request) {
		assetID, err := pathID(r)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_id"})
			return
		}
		documents, err := deps.Documents.DocumentsForObject(r.Context(), store.ObjectTypeAsset, assetID)
		if err != nil {
			deps.Logger.Error("list asset documents", "error", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal_error"})
			return
		}
		writeJSON(w, http.StatusOK, documents)
	})

	mux.HandleFunc("POST /api/v1/assets/{id}/documents", func(w http.ResponseWriter, r *http.Request) {
		assetID, err := pathID(r)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_id"})
			return
		}
		var input struct {
			DocumentID int64 `json:"documentId"`
		}
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil || input.DocumentID == 0 {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_document"})
			return
		}
		if err := deps.Documents.LinkDocument(r.Context(), input.DocumentID, store.ObjectTypeAsset, assetID); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})

	mux.HandleFunc("DELETE /api/v1/assets/{id}/documents/{documentId}", func(w http.ResponseWriter, r *http.Request) {
		assetID, err := pathID(r)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_id"})
			return
		}
		documentID, err := strconv.ParseInt(r.PathValue("documentId"), 10, 64)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_document_id"})
			return
		}
		if err := deps.Documents.UnlinkDocument(r.Context(), documentID, store.ObjectTypeAsset, assetID); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
}
