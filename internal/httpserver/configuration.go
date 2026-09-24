package httpserver

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"hohoho/internal/config"
)

func registerConfigurationRoutes(mux *http.ServeMux, deps Dependencies) {
	mux.HandleFunc("GET /api/v1/system/configuration", func(w http.ResponseWriter, r *http.Request) {
		if deps.Configuration == nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "configuration_unavailable"})
			return
		}
		writeJSON(w, http.StatusOK, deps.Configuration.View())
	})

	mux.HandleFunc("PUT /api/v1/system/configuration", func(w http.ResponseWriter, r *http.Request) {
		if deps.Configuration == nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "configuration_unavailable"})
			return
		}
		var input config.Update
		decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&input); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_json"})
			return
		}
		if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_json"})
			return
		}
		if err := deps.Configuration.Save(input); err != nil {
			switch {
			case errors.Is(err, config.ErrReadOnly):
				writeJSON(w, http.StatusForbidden, map[string]string{"error": "configuration_read_only"})
			case errors.Is(err, config.ErrInvalid):
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_configuration"})
			default:
				deps.Logger.Error("save system configuration", "error", "persistence_failed")
				writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal_error"})
			}
			return
		}
		writeJSON(w, http.StatusOK, deps.Configuration.View())
	})
}
