package store

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// CoreDocument is one uploaded document. Original bytes live in S3/Garage;
// SQLite owns this metadata, the extracted content and the object links.
type CoreDocument struct {
	ID             int64   `json:"id"`
	Filename       string  `json:"filename"`
	MIMEType       string  `json:"mimeType"`
	SizeBytes      int64   `json:"sizeBytes"`
	SHA256         string  `json:"sha256"`
	Status         string  `json:"status"`
	CreatedAt      string  `json:"createdAt"`
	DocumentDate   string  `json:"documentDate,omitempty"`
	Description    string  `json:"description,omitempty"`
	ExtractedText  string  `json:"extractedText,omitempty"`
	TranslatedText string  `json:"translatedText,omitempty"`
	Analysis       any     `json:"analysis,omitempty"`
	ObjectIDs      []int64 `json:"financialObjectIds"`
	ObjectKeys     string  `json:"-"`
}

type SaveCoreDocument struct {
	Filename     string
	MIMEType     string
	SizeBytes    int64
	SHA256       string
	ObjectKey    string
	DocumentDate string
	Description  string
}

func normalizeCoreDocument(input SaveCoreDocument) (SaveCoreDocument, error) {
	input.Filename = strings.TrimSpace(input.Filename)
	input.MIMEType = strings.TrimSpace(input.MIMEType)
	input.ObjectKey = strings.TrimSpace(input.ObjectKey)
	input.DocumentDate = strings.TrimSpace(input.DocumentDate)
	input.Description = strings.TrimSpace(input.Description)
	if input.Filename == "" || input.ObjectKey == "" || input.SizeBytes <= 0 {
		return input, fmt.Errorf("%w: een document heeft een bestandsnaam, een objectkey en inhoud nodig", ErrCoreInvalid)
	}
	if input.DocumentDate != "" {
		parsed, err := time.Parse("2006-01-02", input.DocumentDate)
		if err != nil || parsed.Format("2006-01-02") != input.DocumentDate {
			return input, fmt.Errorf("%w: documentdatum moet YYYY-MM-DD zijn", ErrCoreInvalid)
		}
	}
	return input, nil
}

const coreDocumentColumns = `d.id, d.filename, d.mime_type, d.size_bytes, d.sha256, d.status, d.created_at,
	coalesce(d.document_date, ''), coalesce(d.description, ''), coalesce(d.extracted_text, ''),
	coalesce(d.translated_text, ''), coalesce(d.analysis_json, ''), d.object_key`

// scanCoreDocument leest de twaalf kolommen van coreDocumentColumns plus de
// json_group_array-linkkolom die elke query als dertiende kolom toevoegt.
func scanCoreDocument(scanner interface{ Scan(...any) error }) (CoreDocument, error) {
	var item CoreDocument
	var analysisJSON string
	var linksJSON string
	err := scanner.Scan(&item.ID, &item.Filename, &item.MIMEType, &item.SizeBytes, &item.SHA256, &item.Status, &item.CreatedAt,
		&item.DocumentDate, &item.Description, &item.ExtractedText, &item.TranslatedText, &analysisJSON, &item.ObjectKeys, &linksJSON)
	if analysisJSON != "" {
		var analysis any
		if err := json.Unmarshal([]byte(analysisJSON), &analysis); err == nil {
			item.Analysis = analysis
		}
	}
	item.ObjectIDs = []int64{}
	if linksJSON != "" {
		if err := json.Unmarshal([]byte(linksJSON), &item.ObjectIDs); err != nil {
			return item, err
		}
	}
	return item, coreStoreError(err)
}

func (s *Core) CreateCoreDocument(ctx context.Context, input SaveCoreDocument) (CoreDocument, error) {
	input, err := normalizeCoreDocument(input)
	if err != nil {
		return CoreDocument{}, err
	}
	result, err := s.db.ExecContext(ctx, `INSERT INTO documents (filename, object_key, mime_type, size_bytes, sha256, document_date, description)
		VALUES (?, ?, ?, ?, ?, NULLIF(?, ''), NULLIF(?, ''))`, input.Filename, input.ObjectKey, input.MIMEType, input.SizeBytes, input.SHA256, input.DocumentDate, input.Description)
	if err != nil {
		return CoreDocument{}, coreStoreError(err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return CoreDocument{}, err
	}
	return s.CoreDocument(ctx, id)
}

func (s *Core) CoreDocuments(ctx context.Context) ([]CoreDocument, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+coreDocumentColumns+`,
		coalesce((SELECT json_group_array(l.financial_object_id) FROM document_financial_objects l WHERE l.document_id = d.id), '[]')
		FROM documents d ORDER BY d.created_at DESC, d.id DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []CoreDocument{}
	for rows.Next() {
		item, err := scanCoreDocument(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Core) CoreDocument(ctx context.Context, id int64) (CoreDocument, error) {
	return scanCoreDocument(s.db.QueryRowContext(ctx, `SELECT `+coreDocumentColumns+`,
		coalesce((SELECT json_group_array(l.financial_object_id) FROM document_financial_objects l WHERE l.document_id = d.id), '[]')
		FROM documents d WHERE d.id = ?`, id))
}

func (s *Core) CoreDocumentsForObject(ctx context.Context, objectID int64) ([]CoreDocument, error) {
	if _, err := s.FinancialObject(ctx, objectID); err != nil {
		return nil, err
	}
	rows, err := s.db.QueryContext(ctx, `SELECT `+coreDocumentColumns+`,
		coalesce((SELECT json_group_array(l.financial_object_id) FROM document_financial_objects l WHERE l.document_id = d.id), '[]')
		FROM documents d WHERE EXISTS (SELECT 1 FROM document_financial_objects l WHERE l.document_id = d.id AND l.financial_object_id = ?)
		ORDER BY d.created_at DESC, d.id DESC`, objectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []CoreDocument{}
	for rows.Next() {
		item, err := scanCoreDocument(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Core) LinkCoreDocument(ctx context.Context, documentID, objectID int64) error {
	if _, err := s.CoreDocument(ctx, documentID); err != nil {
		return err
	}
	if _, err := s.FinancialObject(ctx, objectID); err != nil {
		return err
	}
	_, err := s.db.ExecContext(ctx, "INSERT INTO document_financial_objects (document_id, financial_object_id) VALUES (?, ?)", documentID, objectID)
	return coreStoreError(err)
}

func (s *Core) UnlinkCoreDocument(ctx context.Context, documentID, objectID int64) error {
	result, err := s.db.ExecContext(ctx, "DELETE FROM document_financial_objects WHERE document_id = ? AND financial_object_id = ?", documentID, objectID)
	return coreMutationError(result, err)
}

// UpdateCoreDocumentContent maps PDF-AI results back onto the document row.
// It never opens a transaction around external calls; the caller fetches the
// bytes from S3 first and only then stores the outcome here.
func (s *Core) UpdateCoreDocumentContent(ctx context.Context, id int64, extractedText, translatedText, analysisJSON string) error {
	if analysisJSON != "" && !json.Valid([]byte(analysisJSON)) {
		return fmt.Errorf("%w: analyse-resultaat is geen geldige JSON", ErrCoreInvalid)
	}
	result, err := s.db.ExecContext(ctx, `UPDATE documents SET status = 'A', extracted_text = NULLIF(?, ''), translated_text = NULLIF(?, ''), analysis_json = NULLIF(?, '') WHERE id = ?`,
		strings.TrimSpace(extractedText), strings.TrimSpace(translatedText), strings.TrimSpace(analysisJSON), id)
	return coreMutationError(result, err)
}

func (s *Core) UpdateCoreDocumentStatus(ctx context.Context, id int64, status string) error {
	status = strings.TrimSpace(status)
	if status != "I" && status != "A" && status != "F" && status != "U" {
		return fmt.Errorf("%w: documentstatus is ongeldig", ErrCoreInvalid)
	}
	result, err := s.db.ExecContext(ctx, "UPDATE documents SET status = ? WHERE id = ?", status, id)
	return coreMutationError(result, err)
}

// DeleteCoreDocument removes metadata and links only. The original bytes in
// Garage/S3 are removed by the caller through the objectstore.
func (s *Core) DeleteCoreDocument(ctx context.Context, id int64) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, "DELETE FROM document_financial_objects WHERE document_id = ?", id); err != nil {
		return coreStoreError(err)
	}
	result, err := tx.ExecContext(ctx, "DELETE FROM documents WHERE id = ?", id)
	if err := coreMutationError(result, err); err != nil {
		return err
	}
	return tx.Commit()
}
