package httpserver

import (
	"net/http"
	"strconv"

	"hohoho/internal/store"
)

func registerCoreDashboardRoutes(mux *http.ServeMux, deps CoreDependencies) {
	period := func(w http.ResponseWriter, r *http.Request, load func(int, int) (any, error)) {
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
		item, err := load(year, month)
		coreRespond(w, r, deps, http.StatusOK, item, err)
	}
	mux.HandleFunc("GET /api/v1/dashboard", func(w http.ResponseWriter, r *http.Request) {
		period(w, r, func(year, month int) (any, error) { return deps.Store.Dashboard(r.Context(), year, month) })
	})
	mux.HandleFunc("GET /api/v1/budget", func(w http.ResponseWriter, r *http.Request) {
		period(w, r, func(year, month int) (any, error) { return deps.Store.Budget(r.Context(), year, month) })
	})
}
