package httpserver

import (
	"net/http"

	"hohoho/internal/store"
)

func registerDomainEventRoutes(mux *http.ServeMux, deps Dependencies) {
	for path, entityType := range map[string]string{
		"assets":       store.ObjectTypeAsset,
		"liabilities":  store.ObjectTypeLiability,
		"reservations": store.ObjectTypeReservation,
		"cases":        store.ObjectTypeCase,
	} {
		mux.HandleFunc("GET /api/v1/"+path+"/{id}/events", func(w http.ResponseWriter, r *http.Request) {
			id, err := pathID(r)
			if err != nil || id <= 0 {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_id"})
				return
			}
			items, err := deps.Events.DomainEventsForObject(r.Context(), entityType, id)
			if err != nil {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
				return
			}
			writeJSON(w, http.StatusOK, items)
		})
	}
}

func registerUnavailableDomainEventRoutes(mux *http.ServeMux) {
	unavailable := func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusNotImplemented, map[string]string{"error": "domain_events_require_postgres"})
	}
	mux.HandleFunc("GET /api/v1/assets/{id}/events", unavailable)
	mux.HandleFunc("GET /api/v1/liabilities/{id}/events", unavailable)
	mux.HandleFunc("GET /api/v1/reservations/{id}/events", unavailable)
	mux.HandleFunc("GET /api/v1/cases/{id}/events", unavailable)
}
