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
	AnalyzeDocument(ctx context.Context, document store.CoreDocument) (CoreDocumentAnalysisResult, error)
}

type CoreDocumentAnalysisResult struct {
	ExtractedText      string
	TranslatedText     string
	AnalysisJSON       string
	FinancialObjectIDs []int64
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
		if err == nil {
			startCoreDocumentAnalysis(deps, item)
		}
		coreRespond(w, r, deps, http.StatusCreated, item, err)
	})
	mux.HandleFunc("POST /api/v1/documents/analyze-batch", func(w http.ResponseWriter, r *http.Request) {
		if deps.DocumentAnalyzer == nil || deps.DocumentAnalysisQueue == nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "Documentanalyse is niet geconfigureerd"})
			return
		}
		items, err := deps.Store.CoreDocuments(r.Context())
		if err != nil {
			coreRespond(w, r, deps, 0, nil, err)
			return
		}
		batch := make([]store.CoreDocument, 0, len(items))
		for _, item := range items {
			if item.Status == "I" || item.Status == "F" {
				batch = append(batch, item)
			}
		}
		go func(documents []store.CoreDocument) {
			for _, document := range documents {
				deps.DocumentAnalysisQueue <- document
			}
		}(batch)
		writeJSON(w, http.StatusAccepted, map[string]int{"queued": len(batch)})
	})
	mux.HandleFunc("POST /api/v1/documents/analyze-selected", func(w http.ResponseWriter, r *http.Request) {
		if deps.DocumentAnalyzer == nil || deps.DocumentAnalysisQueue == nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "Documentanalyse is niet geconfigureerd"})
			return
		}
		var input struct {
			DocumentIDs []int64 `json:"documentIds"`
		}
		if !coreDecode(w, r, &input) {
			return
		}
		queued := 0
		seen := map[int64]bool{}
		for _, id := range input.DocumentIDs {
			if id <= 0 || seen[id] {
				continue
			}
			seen[id] = true
			document, err := deps.Store.CoreDocument(r.Context(), id)
			if err != nil {
				if errors.Is(err, store.ErrCoreNotFound) {
					continue
				}
				coreRespond(w, r, deps, 0, nil, err)
				return
			}
			startCoreDocumentAnalysis(deps, document)
			queued++
		}
		writeJSON(w, http.StatusAccepted, map[string]int{"queued": queued})
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
		result, err := analyzer.AnalyzeDocument(analysisCtx, document)
		if err != nil {
			if statusErr := deps.Store.UpdateCoreDocumentStatus(context.WithoutCancel(r.Context()), id, "F"); statusErr != nil {
				deps.Logger.Error("documentstatus opslaan mislukt", "document_id", id, "error", statusErr)
			}
			deps.Logger.Error("documentanalyse mislukt", "document_id", id, "error", err)
			writeJSON(w, http.StatusBadGateway, map[string]string{"error": "Documentanalyse is mislukt"})
			return
		}
		if err := deps.Store.UpdateCoreDocumentContent(context.WithoutCancel(r.Context()), id, result.ExtractedText, result.TranslatedText, result.AnalysisJSON); err != nil {
			coreRespond(w, r, deps, 0, nil, err)
			return
		}
		if err := linkAnalyzedDocumentObjects(context.WithoutCancel(r.Context()), deps, id, result.FinancialObjectIDs); err != nil {
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

func linkAnalyzedDocumentObjects(ctx context.Context, deps CoreDependencies, documentID int64, objectIDs []int64) error {
	if len(objectIDs) == 0 {
		return nil
	}
	seen := map[int64]bool{}
	for _, objectID := range objectIDs {
		if objectID <= 0 || seen[objectID] {
			continue
		}
		seen[objectID] = true
		if _, err := deps.Store.FinancialObject(ctx, objectID); err != nil {
			if errors.Is(err, store.ErrCoreNotFound) {
				continue
			}
			return err
		}
		if err := deps.Store.LinkCoreDocument(ctx, documentID, objectID); err != nil && !errors.Is(err, store.ErrCoreConflict) {
			return err
		}
	}
	return nil
}
