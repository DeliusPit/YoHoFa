package httpserver

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"hohoho/internal/config"
	"hohoho/internal/maintenance"
	"hohoho/internal/store"
	"hohoho/internal/systembackup"
)

var errObjectStorageUnavailable = errors.New("Documentopslag is niet beschikbaar")

type CoreDependencies struct {
	Config       config.Runtime
	Store        *store.Core
	Objects      *CoreObjectStorage
	SystemBackup *systembackup.Service
	Logger       *slog.Logger
	Maintenance  *maintenance.Gate
	// DocumentAnalyzer and DocumentAnalysisQueue detach document AI from storage.
	// Upload works fully without them; when set, uploads are queued for analysis.
	DocumentAnalyzer      CoreDocumentAnalyzer
	DocumentAnalysisQueue chan store.CoreDocument
}

func NewCore(deps CoreDependencies) http.Handler {
	if deps.Logger == nil {
		deps.Logger = slog.Default()
	}
	if deps.Objects == nil {
		deps.Objects = NewCoreObjectStorage()
	}
	mux := http.NewServeMux()
	if deps.DocumentAnalysisQueue == nil && deps.DocumentAnalyzer != nil {
		deps.DocumentAnalysisQueue = make(chan store.CoreDocument, 16)
	}
	registerCoreDocumentRoutes(mux, deps)
	registerCoreTransactionRoutes(mux, deps)
	registerCoreDashboardRoutes(mux, deps)
	registerCoreConfigurationRoutes(mux, deps)
	registerCoreBankConfigRoutes(mux, deps)
	registerCoreBankImportRoutes(mux, deps)
	registerSystemBackupRoutes(mux, deps)
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		if err := deps.Store.Ping(r.Context()); err != nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "unhealthy"})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "database": "sqlite"})
	})
	mux.HandleFunc("GET /readyz", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
		defer cancel()
		if deps.Store.Ping(ctx) != nil || deps.Objects.Ping(ctx) != nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "not_ready"})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	mux.HandleFunc("GET /api/v1/summary", func(w http.ResponseWriter, r *http.Request) {
		item, err := deps.Store.CoreSummary(r.Context())
		coreRespond(w, r, deps, http.StatusOK, item, err)
	})
	mux.HandleFunc("GET /api/v1/financial-objects", func(w http.ResponseWriter, r *http.Request) {
		items, err := deps.Store.FinancialObjects(r.Context(), r.URL.Query().Get("type"))
		coreRespond(w, r, deps, http.StatusOK, items, err)
	})
	mux.HandleFunc("POST /api/v1/financial-objects", func(w http.ResponseWriter, r *http.Request) {
		var input store.SaveFinancialObject
		if !coreDecode(w, r, &input) {
			return
		}
		item, err := deps.Store.CreateFinancialObject(r.Context(), input)
		coreRespond(w, r, deps, http.StatusCreated, item, err)
	})
	mux.HandleFunc("GET /api/v1/financial-objects/{id}", coreObjectHandler(func(w http.ResponseWriter, r *http.Request, id int64) {
		item, err := deps.Store.FinancialObject(r.Context(), id)
		coreRespond(w, r, deps, http.StatusOK, item, err)
	}))
	mux.HandleFunc("PUT /api/v1/financial-objects/{id}", coreObjectHandler(func(w http.ResponseWriter, r *http.Request, id int64) {
		var input store.SaveFinancialObject
		if !coreDecode(w, r, &input) {
			return
		}
		item, err := deps.Store.UpdateFinancialObject(r.Context(), id, input)
		coreRespond(w, r, deps, http.StatusOK, item, err)
	}))
	mux.HandleFunc("DELETE /api/v1/financial-objects/{id}", coreObjectHandler(func(w http.ResponseWriter, r *http.Request, id int64) {
		err := deps.Store.DeleteFinancialObject(r.Context(), id)
		coreRespond(w, r, deps, http.StatusNoContent, nil, err)
	}))
	mux.HandleFunc("GET /api/v1/financial-objects/{id}/values", coreObjectHandler(func(w http.ResponseWriter, r *http.Request, id int64) {
		items, err := deps.Store.ObjectValues(r.Context(), id)
		coreRespond(w, r, deps, http.StatusOK, items, err)
	}))
	mux.HandleFunc("POST /api/v1/financial-objects/{id}/values", coreObjectHandler(func(w http.ResponseWriter, r *http.Request, id int64) {
		var input store.SaveObjectValue
		if !coreDecode(w, r, &input) {
			return
		}
		item, err := deps.Store.CreateObjectValue(r.Context(), id, input)
		coreRespond(w, r, deps, http.StatusCreated, item, err)
	}))
	mux.HandleFunc("DELETE /api/v1/financial-objects/{id}/values/{valueId}", coreObjectHandler(func(w http.ResponseWriter, r *http.Request, id int64) {
		valueID, err := strconv.ParseInt(r.PathValue("valueId"), 10, 64)
		if err != nil || valueID <= 0 {
			coreRespond(w, r, deps, 0, nil, store.ErrCoreInvalid)
			return
		}
		err = deps.Store.DeleteObjectValue(r.Context(), id, valueID)
		coreRespond(w, r, deps, http.StatusNoContent, nil, err)
	}))
	mux.HandleFunc("GET /api/v1/financial-objects/{id}/relations", coreObjectHandler(func(w http.ResponseWriter, r *http.Request, id int64) {
		items, err := deps.Store.ObjectRelations(r.Context(), id)
		coreRespond(w, r, deps, http.StatusOK, items, err)
	}))
	mux.HandleFunc("POST /api/v1/object-relations", func(w http.ResponseWriter, r *http.Request) {
		var input store.ObjectRelation
		if !coreDecode(w, r, &input) {
			return
		}
		item, err := deps.Store.CreateObjectRelation(r.Context(), input)
		coreRespond(w, r, deps, http.StatusCreated, item, err)
	})
	mux.HandleFunc("DELETE /api/v1/object-relations", func(w http.ResponseWriter, r *http.Request) {
		var input store.ObjectRelation
		if !coreDecode(w, r, &input) {
			return
		}
		err := deps.Store.DeleteObjectRelation(r.Context(), input)
		coreRespond(w, r, deps, http.StatusNoContent, nil, err)
	})
	// Unknown/legacy API paths must never fall through to the SPA's index.html.
	mux.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "API-route niet gevonden"})
	})
	if deps.Config.WebDir != "" {
		static := staticHandler(deps.Config.WebDir)
		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodGet && r.Method != http.MethodHead {
				w.Header().Set("Allow", "GET, HEAD")
				http.Error(w, "Methode niet toegestaan", http.StatusMethodNotAllowed)
				return
			}
			static(w, r)
		})
	}
	var handler http.Handler = mux
	if deps.Maintenance != nil {
		handler = maintenanceGuard(deps.Maintenance, handler)
	}
	if deps.Config.AuthUsername != "" && deps.Config.AuthPassword != "" {
		handler = basicAuth(deps.Config.AuthUsername, deps.Config.AuthPassword, handler)
	}
	if deps.Config.CORSOrigin != "" {
		handler = cors(deps.Config.CORSOrigin, handler)
	}
	return handler
}

func maintenanceGuard(gate *maintenance.Gate, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
			release := gate.ReadLock()
			defer release()
		}
		next.ServeHTTP(w, r)
	})
}

// RunCoreDocumentAnalysis consumes the upload queue until the context ends.
// It runs outside every database transaction and outside the request handlers.
func RunCoreDocumentAnalysis(ctx context.Context, deps CoreDependencies) {
	if deps.DocumentAnalyzer == nil || deps.DocumentAnalysisQueue == nil {
		return
	}
	for {
		select {
		case <-ctx.Done():
			return
		case document := <-deps.DocumentAnalysisQueue:
			extractedText, translatedText, analysisJSON, err := deps.DocumentAnalyzer.AnalyzeDocument(ctx, document)
			if err != nil {
				deps.Logger.Error("automatische documentanalyse mislukt", "document_id", document.ID, "error", err)
				continue
			}
			var release func()
			if deps.Maintenance != nil {
				release = deps.Maintenance.ReadLock()
			}
			err = deps.Store.UpdateCoreDocumentContent(ctx, document.ID, extractedText, translatedText, analysisJSON)
			if release != nil {
				release()
			}
			if err != nil {
				deps.Logger.Error("documentanalyse-resultaten opslaan mislukt", "document_id", document.ID, "error", err)
			}
		}
	}
}

func coreObjectHandler(next func(http.ResponseWriter, *http.Request, int64)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil || id <= 0 {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Ongeldig ID"})
			return
		}
		next(w, r, id)
	}
}

func coreDecode(w http.ResponseWriter, r *http.Request, input any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(input); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Ongeldige JSON-invoer"})
		return false
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Verwacht één JSON-object"})
		return false
	}
	return true
}

func coreRespond(w http.ResponseWriter, r *http.Request, deps CoreDependencies, status int, item any, err error) {
	if err != nil {
		switch {
		case errors.Is(err, store.ErrCoreInvalid):
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		case errors.Is(err, store.ErrCoreNotFound):
			writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		case errors.Is(err, store.ErrCoreConflict):
			writeJSON(w, http.StatusConflict, map[string]string{"error": err.Error()})
		case errors.Is(err, errObjectStorageUnavailable):
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": err.Error()})
		default:
			deps.Logger.Error("Databasebewerking mislukt", "path", r.URL.Path, "error", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Interne fout"})
		}
		return
	}
	if status == http.StatusNoContent {
		w.WriteHeader(status)
		return
	}
	writeJSON(w, status, item)
}
