package httpserver

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"hohoho/internal/store"
)

const maxDocumentSize = 25 << 20

func registerDocumentRoutes(mux *http.ServeMux, deps Dependencies) {
	mux.HandleFunc("GET /api/v1/documents", func(w http.ResponseWriter, r *http.Request) {
		documents, err := deps.Documents.Documents(r.Context())
		if err != nil {
			deps.Logger.Error("list documents", "error", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal_error"})
			return
		}
		writeJSON(w, http.StatusOK, documents)
	})

	mux.HandleFunc("POST /api/v1/documents", func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, maxDocumentSize+(1<<20))
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_or_too_large_upload"})
			return
		}
		file, header, err := r.FormFile("file")
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "missing_file"})
			return
		}
		defer file.Close()

		data, err := io.ReadAll(io.LimitReader(file, maxDocumentSize+1))
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "read_upload_failed"})
			return
		}
		if len(data) == 0 || len(data) > maxDocumentSize {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_or_too_large_upload"})
			return
		}

		contentType := http.DetectContentType(data)
		documentDate := strings.TrimSpace(r.FormValue("documentDate"))
		if documentDate != "" {
			if _, err := time.Parse("2006-01-02", documentDate); err != nil {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_document_date"})
				return
			}
		}
		objectKey, err := randomObjectKey()
		if err != nil {
			deps.Logger.Error("generate object key", "error", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal_error"})
			return
		}
		checksum := sha256.Sum256(data)
		if err := deps.Objects.Put(r.Context(), objectKey, data, contentType); err != nil {
			deps.Logger.Error("store document object", "error", err)
			writeJSON(w, http.StatusBadGateway, map[string]string{"error": "object_storage_failed"})
			return
		}

		document, err := deps.Documents.CreateDocument(r.Context(), store.CreateDocument{
			OriginalFilename: filepath.Base(header.Filename),
			MIMEType:         contentType,
			SizeBytes:        int64(len(data)),
			SHA256:           hex.EncodeToString(checksum[:]),
			ObjectKey:        objectKey,
			DocumentDate:     documentDate,
			Description:      strings.TrimSpace(r.FormValue("description")),
		})
		if err != nil {
			cleanupCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			if cleanupErr := deps.Objects.Delete(cleanupCtx, objectKey); cleanupErr != nil {
				deps.Logger.Error("cleanup document object", "error", cleanupErr)
			}
			deps.Logger.Error("store document metadata", "error", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "metadata_storage_failed"})
			return
		}
		if deps.Config.LLMDocumentMetadata && deps.DocumentTranslation != nil {
			deps.DocumentTranslation.Start(document.ID)
			document.TranslationStatus = "pending"
		}
		writeJSON(w, http.StatusCreated, document)
	})

	mux.HandleFunc("GET /api/v1/documents/{id}/content", func(w http.ResponseWriter, r *http.Request) {
		id, err := pathID(r)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_id"})
			return
		}
		document, err := deps.Documents.Document(r.Context(), id)
		if err != nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "document_not_found"})
			return
		}
		object, err := deps.Objects.Get(r.Context(), document.ObjectKey)
		if err != nil {
			deps.Logger.Error("read document object", "document_id", id, "error", err)
			writeJSON(w, http.StatusBadGateway, map[string]string{"error": "object_storage_failed"})
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
		w.Header().Set("Content-Disposition", mime.FormatMediaType(disposition, map[string]string{"filename": document.OriginalFilename}))
		if _, err := io.Copy(w, object); err != nil {
			deps.Logger.Error("stream document object", "document_id", id, "error", err)
		}
	})

	mux.HandleFunc("GET /api/v1/transactions/{id}/documents", func(w http.ResponseWriter, r *http.Request) {
		transactionID, err := pathID(r)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_id"})
			return
		}
		documents, err := deps.Documents.DocumentsForObject(r.Context(), store.ObjectTypeTransaction, transactionID)
		if err != nil {
			deps.Logger.Error("list transaction documents", "error", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal_error"})
			return
		}
		writeJSON(w, http.StatusOK, documents)
	})

	mux.HandleFunc("POST /api/v1/transactions/{id}/documents", func(w http.ResponseWriter, r *http.Request) {
		transactionID, err := pathID(r)
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
		if err := deps.Documents.LinkDocument(r.Context(), input.DocumentID, store.ObjectTypeTransaction, transactionID); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})

	mux.HandleFunc("DELETE /api/v1/transactions/{id}/documents/{documentId}", func(w http.ResponseWriter, r *http.Request) {
		transactionID, err := pathID(r)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_id"})
			return
		}
		documentID, err := strconv.ParseInt(r.PathValue("documentId"), 10, 64)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_document_id"})
			return
		}
		if err := deps.Documents.UnlinkDocument(r.Context(), documentID, store.ObjectTypeTransaction, transactionID); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
}

func safeInlineDocumentType(contentType string) bool {
	switch contentType {
	case "application/pdf", "image/gif", "image/jpeg", "image/png", "image/webp":
		return true
	default:
		return false
	}
}

func randomObjectKey() (string, error) {
	value := make([]byte, 32)
	if _, err := rand.Read(value); err != nil {
		return "", fmt.Errorf("generate random key: %w", err)
	}
	return hex.EncodeToString(value), nil
}
