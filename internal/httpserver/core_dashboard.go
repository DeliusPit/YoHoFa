package httpserver

import (
	"net/http"
	"strconv"

	"hohoho/internal/store"
)

func registerCoreDashboardRoutes(mux *http.ServeMux, deps CoreDependencies) {
	mux.HandleFunc("GET /api/v1/dashboard", func(w http.ResponseWriter, r *http.Request) {
		year, month := 0, 0
		for key, destination := range map[string]*int{"year": &year, "month": &month} {
			if value := r.URL.Query().Get(key); value != "" {
				parsed, err := strconv.Atoi(value)
				if err != nil || (key == "year" && parsed <= 0) {
					coreRespond(w, r, deps, 0, nil, store.ErrCoreInvalid)
					return
				}
				*destination = parsed
			}
		}
		item, err := deps.Store.Dashboard(r.Context(), year, month)
		coreRespond(w, r, deps, http.StatusOK, item, err)
	})
}
