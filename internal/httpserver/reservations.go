package httpserver

import (
	"encoding/json"
	"net/http"
	"strconv"

	"hohoho/internal/store"
)

func registerReservationRoutes(mux *http.ServeMux, deps Dependencies) {
	mux.HandleFunc("GET /api/v1/reservations", func(w http.ResponseWriter, r *http.Request) {
		items, err := deps.Reservations.Reservations(r.Context(), store.ReservationFilter{ReserveType: r.URL.Query().Get("type")})
		if err != nil {
			deps.Logger.Error("list reservations", "error", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal_error"})
			return
		}
		writeJSON(w, http.StatusOK, items)
	})
	mux.HandleFunc("POST /api/v1/reservations", func(w http.ResponseWriter, r *http.Request) {
		var input store.SaveReservation
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_json"})
			return
		}
		item, err := deps.Reservations.CreateReservation(r.Context(), input)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusCreated, item)
	})
	mux.HandleFunc("GET /api/v1/reservations/{id}", func(w http.ResponseWriter, r *http.Request) {
		id, err := pathID(r)
		if err != nil || id <= 0 {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_id"})
			return
		}
		item, err := deps.Reservations.Reservation(r.Context(), id)
		if err != nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "reservation_not_found"})
			return
		}
		writeJSON(w, http.StatusOK, item)
	})
	mux.HandleFunc("PATCH /api/v1/reservations/{id}", func(w http.ResponseWriter, r *http.Request) {
		id, err := pathID(r)
		if err != nil || id <= 0 {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_id"})
			return
		}
		var input store.SaveReservation
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_json"})
			return
		}
		item, err := deps.Reservations.UpdateReservation(r.Context(), id, input)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, item)
	})
	mux.HandleFunc("DELETE /api/v1/reservations/{id}", func(w http.ResponseWriter, r *http.Request) {
		id, err := pathID(r)
		if err != nil || id <= 0 {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_id"})
			return
		}
		if err := deps.Reservations.DeleteReservation(r.Context(), id); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
}

func registerReservationDocumentRoutes(mux *http.ServeMux, deps Dependencies) {
	mux.HandleFunc("GET /api/v1/reservations/{id}/documents", func(w http.ResponseWriter, r *http.Request) {
		id, err := pathID(r)
		if err != nil || id <= 0 {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_id"})
			return
		}
		items, err := deps.Documents.DocumentsForObject(r.Context(), store.ObjectTypeReservation, id)
		if err != nil {
			deps.Logger.Error("list reservation documents", "error", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal_error"})
			return
		}
		writeJSON(w, http.StatusOK, items)
	})
	mux.HandleFunc("POST /api/v1/reservations/{id}/documents", func(w http.ResponseWriter, r *http.Request) {
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
		if err := deps.Documents.LinkDocument(r.Context(), input.DocumentID, store.ObjectTypeReservation, id); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("DELETE /api/v1/reservations/{id}/documents/{documentId}", func(w http.ResponseWriter, r *http.Request) {
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
		if err := deps.Documents.UnlinkDocument(r.Context(), documentID, store.ObjectTypeReservation, id); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
}
