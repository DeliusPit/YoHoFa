package httpserver

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"mime"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"hohoho/internal/store"
)

const maxCoreDocumentSize = 25 << 20

// CoreDocumentAnalyzer is the deliberately detached AI hook. Storage and this
// interface are separate concerns: upload works fully without any analyzer,
// and an analyzer only receives a read-only view of the stored document.
type CoreDocumentAnalyzer interface {
	AnalyzeDocument(ctx context.Context, document store.CoreDocument) (extractedText string, translatedText string, analysisJSON string, err error)
}

func registerCoreDocumentRoutes(mux *http.ServeMux, deps CoreDependencies) {
	mux.HandleFunc("GET /api/v1/documents", func(w http.ResponseWriter, r *http.Request) {
		if raw := r.URL.Query().Get("financialObjectId"); raw != "" {
			objectID, err := strconv.ParseInt(raw, 10, 64)
			if err != nil || objectID <= 0 {
				coreRespond(w, r, deps, 0, nil, store.ErrCoreInvalid)
				return
			}
			items, err := deps.Store.CoreDocumentsForObject(r.Context(), objectID)
			coreRespond(w, r, deps, http.StatusOK, items, err)
			return
		}
		items, err := deps.Store.CoreDocuments(r.Context())
		coreRespond(w, r, deps, http.StatusOK, items, err)
	})
	mux.HandleFunc("POST /api/v1/documents", func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, maxCoreDocumentSize+(1<<20))
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Upload kon niet worden gelezen of is te groot"})
			return
		}
		file, header, err := r.FormFile("file")
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Kies een bestand"})
			return
		}
		defer file.Close()
		data, err := io.ReadAll(io.LimitReader(file, maxCoreDocumentSize+1))
		if err != nil || len(data) == 0 || len(data) > maxCoreDocumentSize {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Ongeldig of te groot bestand (maximaal 25 MiB)"})
			return
		}
		contentType := http.DetectContentType(data)
		checksum := sha256.Sum256(data)
		objectKey, err := randomObjectKey()
		if err != nil {
			deps.Logger.Error("genereer objectkey mislukt", "error", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Interne fout"})
			return
		}
		if err := deps.Objects.Put(r.Context(), objectKey, data, contentType); err != nil {
			deps.Logger.Error("document naar S3 schrijven mislukt", "error", err)
			writeJSON(w, http.StatusBadGateway, map[string]string{"error": "Documentopslag is niet beschikbaar"})
			return
		}
		input := store.SaveCoreDocument{
			Filename:     filepath.Base(header.Filename),
			MIMEType:     contentType,
			SizeBytes:    int64(len(data)),
			SHA256:       hex.EncodeToString(checksum[:]),
			ObjectKey:    objectKey,
			DocumentDate: strings.TrimSpace(r.FormValue("documentDate")),
			Description:  strings.TrimSpace(r.FormValue("description")),
		}
		document, err := deps.Store.CreateCoreDocument(r.Context(), input)
		if err != nil {
			if cleanupErr := deps.Objects.Delete(context.WithoutCancel(r.Context()), objectKey); cleanupErr != nil {
				deps.Logger.Error("opschonen documentobject mislukt", "object_key", objectKey, "error", cleanupErr)
			}
			coreRespond(w, r, deps, 0, nil, err)
			return
		}
		item, err := deps.Store.CoreDocument(r.Context(), document.ID)
		coreRespond(w, r, deps, http.StatusCreated, item, err)
	})
	mux.HandleFunc("GET /api/v1/documents/{id}", coreObjectHandler(func(w http.ResponseWriter, r *http.Request, id int64) {
		item, err := deps.Store.CoreDocument(r.Context(), id)
		coreRespond(w, r, deps, http.StatusOK, item, err)
	}))
	mux.HandleFunc("DELETE /api/v1/documents/{id}", coreObjectHandler(func(w http.ResponseWriter, r *http.Request, id int64) {
		document, err := deps.Store.CoreDocument(r.Context(), id)
		if err != nil {
			coreRespond(w, r, deps, 0, nil, err)
			return
		}
		if err := deps.Store.DeleteCoreDocument(r.Context(), id); err != nil {
			coreRespond(w, r, deps, 0, nil, err)
			return
		}
		if err := deps.Objects.Delete(context.WithoutCancel(r.Context()), document.ObjectKeys); err != nil && !errors.Is(err, errObjectStorageUnavailable) {
			deps.Logger.Error("documentobject verwijderen uit S3 mislukt", "object_key", document.ObjectKeys, "error", err)
		}
		writeJSON(w, http.StatusNoContent, nil)
	}))
	mux.HandleFunc("GET /api/v1/documents/{id}/content", coreObjectHandler(func(w http.ResponseWriter, r *http.Request, id int64) {
		document, err := deps.Store.CoreDocument(r.Context(), id)
		if err != nil {
			coreRespond(w, r, deps, 0, nil, err)
			return
		}
		if deps.Objects == nil {
			coreRespond(w, r, deps, 0, nil, errObjectStorageUnavailable)
			return
		}
		object, err := deps.Objects.Get(r.Context(), document.ObjectKeys)
		if err != nil {
			deps.Logger.Error("documentobject lezen uit S3 mislukt", "document_id", id, "error", err)
			writeJSON(w, http.StatusBadGateway, map[string]string{"error": "Documentopslag is niet beschikbaar"})
			return
		}
		defer object.Close()
		disposition := "attachment"
		if safeInlineDocumentType(document.MIMEType) {
			disposition = "inline"
		}
		w.Header().Set("Content-Type", document.MIMEType)
		w.Header().Set("Content-Security-Policy", "sandbox; default-src 'none'")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Content-Length", strconv.FormatInt(document.SizeBytes, 10))
		w.Header().Set("Content-Disposition", mime.FormatMediaType(disposition, map[string]string{"filename": document.Filename}))
		if _, err := io.Copy(w, object); err != nil {
			deps.Logger.Error("documentobject streamen mislukt", "document_id", id, "error", err)
		}
	}))
	mux.HandleFunc("POST /api/v1/documents/{id}/links", coreObjectHandler(func(w http.ResponseWriter, r *http.Request, id int64) {
		var input struct {
			FinancialObjectID int64 `json:"financialObjectId"`
		}
		if !coreDecode(w, r, &input) {
			return
		}
		if input.FinancialObjectID <= 0 {
			coreRespond(w, r, deps, 0, nil, store.ErrCoreInvalid)
			return
		}
		if err := deps.Store.LinkCoreDocument(r.Context(), id, input.FinancialObjectID); err != nil {
			coreRespond(w, r, deps, 0, nil, err)
			return
		}
		item, err := deps.Store.CoreDocument(r.Context(), id)
		coreRespond(w, r, deps, http.StatusCreated, item, err)
	}))
	mux.HandleFunc("DELETE /api/v1/documents/{id}/links/{objectId}", coreObjectHandler(func(w http.ResponseWriter, r *http.Request, id int64) {
		objectID, err := strconv.ParseInt(r.PathValue("objectId"), 10, 64)
		if err != nil || objectID <= 0 {
			coreRespond(w, r, deps, 0, nil, store.ErrCoreInvalid)
			return
		}
		err = deps.Store.UnlinkCoreDocument(r.Context(), id, objectID)
		coreRespond(w, r, deps, http.StatusNoContent, nil, err)
	}))
	mux.HandleFunc("POST /api/v1/documents/{id}/analyze", coreObjectHandler(func(w http.ResponseWriter, r *http.Request, id int64) {
		document, err := deps.Store.CoreDocument(r.Context(), id)
		if err != nil {
			coreRespond(w, r, deps, 0, nil, err)
			return
		}
		analyzer := deps.DocumentAnalyzer
		if analyzer == nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "Documentanalyse is niet geconfigureerd"})
			return
		}
		analysisCtx, cancel := context.WithTimeout(context.WithoutCancel(r.Context()), 30*time.Minute)
		defer cancel()
		extractedText, translatedText, analysisJSON, err := analyzer.AnalyzeDocument(analysisCtx, document)
		if err != nil {
			if statusErr := deps.Store.UpdateCoreDocumentStatus(context.WithoutCancel(r.Context()), id, "F"); statusErr != nil {
				deps.Logger.Error("documentstatus opslaan mislukt", "document_id", id, "error", statusErr)
			}
			deps.Logger.Error("documentanalyse mislukt", "document_id", id, "error", err)
			writeJSON(w, http.StatusBadGateway, map[string]string{"error": "Documentanalyse is mislukt"})
			return
		}
		if err := deps.Store.UpdateCoreDocumentContent(context.WithoutCancel(r.Context()), id, extractedText, translatedText, analysisJSON); err != nil {
			coreRespond(w, r, deps, 0, nil, err)
			return
		}
		item, err := deps.Store.CoreDocument(context.WithoutCancel(r.Context()), id)
		coreRespond(w, r, deps, http.StatusOK, item, err)
	}))
	mux.HandleFunc("POST /api/v1/documents/{id}/classify/manual", coreObjectHandler(func(w http.ResponseWriter, r *http.Request, id int64) {
		if err := deps.Store.UpdateCoreDocumentStatus(r.Context(), id, "U"); err != nil {
			coreRespond(w, r, deps, 0, nil, err)
			return
		}
		item, err := deps.Store.CoreDocument(r.Context(), id)
		coreRespond(w, r, deps, http.StatusOK, item, err)
	}))
}

func startCoreDocumentAnalysis(deps CoreDependencies, document store.CoreDocument) {
	if deps.DocumentAnalyzer == nil || deps.DocumentAnalysisQueue == nil {
		return
	}
	select {
	case deps.DocumentAnalysisQueue <- document:
	default:
		deps.Logger.Warn("wachtrij voor documentanalyse is vol; analyse overgeslagen", "document_id", document.ID)
	}
}
