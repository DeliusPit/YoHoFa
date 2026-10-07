package httpserver

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"hohoho/internal/systembackup"
)

func registerSystemBackupRoutes(mux *http.ServeMux, deps CoreDependencies) {
	mux.HandleFunc("GET /api/v1/system/backup", func(w http.ResponseWriter, r *http.Request) {
		if deps.SystemBackup == nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "backup_unavailable"})
			return
		}
		writeJSON(w, http.StatusOK, deps.SystemBackup.View())
	})

	mux.HandleFunc("PUT /api/v1/system/backup", func(w http.ResponseWriter, r *http.Request) {
		if deps.SystemBackup == nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "backup_unavailable"})
			return
		}
		var input systembackup.SettingsInput
		decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 256<<10))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&input); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_json"})
			return
		}
		if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_json"})
			return
		}
		view, err := deps.SystemBackup.SaveAndValidate(r.Context(), input)
		if err != nil {
			status := http.StatusBadGateway
			if errors.Is(err, systembackup.ErrRunning) {
				status = http.StatusConflict
			} else if errors.Is(err, systembackup.ErrInvalid) {
				status = http.StatusBadRequest
			}
			writeJSON(w, status, map[string]string{"error": systembackup.ErrorCode(err)})
			return
		}
		writeJSON(w, http.StatusOK, view)
	})

	mux.HandleFunc("POST /api/v1/system/backup/validate", func(w http.ResponseWriter, r *http.Request) {
		if deps.SystemBackup == nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "backup_unavailable"})
			return
		}
		view, err := deps.SystemBackup.Validate(r.Context())
		if err != nil {
			status := http.StatusBadGateway
			if errors.Is(err, systembackup.ErrRunning) {
				status = http.StatusConflict
			}
			writeJSON(w, status, map[string]string{"error": systembackup.ErrorCode(err)})
			return
		}
		writeJSON(w, http.StatusOK, view)
	})

	mux.HandleFunc("POST /api/v1/system/backup/runs", func(w http.ResponseWriter, r *http.Request) {
		if deps.SystemBackup == nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "backup_unavailable"})
			return
		}
		if err := deps.SystemBackup.Start(); err != nil {
			switch {
			case errors.Is(err, systembackup.ErrNotReady):
				writeJSON(w, http.StatusPreconditionFailed, map[string]string{"error": "backup_not_ready"})
			case errors.Is(err, systembackup.ErrRunning):
				writeJSON(w, http.StatusConflict, map[string]string{"error": "backup_running"})
			default:
				writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal_error"})
			}
			return
		}
		writeJSON(w, http.StatusAccepted, deps.SystemBackup.View())
	})

	mux.HandleFunc("GET /api/v1/system/backup/snapshots", func(w http.ResponseWriter, r *http.Request) {
		if deps.SystemBackup == nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "backup_unavailable"})
			return
		}
		items, err := deps.SystemBackup.Snapshots(r.Context())
		if err != nil {
			status := http.StatusBadGateway
			if errors.Is(err, systembackup.ErrRunning) {
				status = http.StatusConflict
			} else if errors.Is(err, systembackup.ErrInvalid) {
				status = http.StatusPreconditionFailed
			}
			writeJSON(w, status, map[string]string{"error": systembackup.ErrorCode(err)})
			return
		}
		writeJSON(w, http.StatusOK, items)
	})

	mux.HandleFunc("POST /api/v1/system/backup/recovery", func(w http.ResponseWriter, r *http.Request) {
		if deps.SystemBackup == nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "backup_unavailable"})
			return
		}
		var input systembackup.RecoveryInput
		decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 32<<10))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&input); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_json"})
			return
		}
		if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_json"})
			return
		}
		if err := deps.SystemBackup.ScheduleRecovery(r.Context(), input); err != nil {
			status := http.StatusBadGateway
			if errors.Is(err, systembackup.ErrInvalid) || errors.Is(err, systembackup.ErrConfirmation) {
				status = http.StatusBadRequest
			} else if errors.Is(err, systembackup.ErrSnapshotNotFound) {
				status = http.StatusNotFound
			} else if errors.Is(err, systembackup.ErrRunning) {
				status = http.StatusConflict
			}
			writeJSON(w, status, map[string]string{"error": systembackup.ErrorCode(err)})
			return
		}
		writeJSON(w, http.StatusAccepted, deps.SystemBackup.View())
	})
}
