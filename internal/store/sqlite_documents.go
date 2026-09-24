package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

func (s *SQLite) CreateDocument(ctx context.Context, input CreateDocument) (Document, error) {
	var documentDate any
	if input.DocumentDate != "" {
		if _, err := time.Parse("2006-01-02", input.DocumentDate); err != nil {
			return Document{}, fmt.Errorf("invalid document date")
		}
		documentDate = input.DocumentDate
	}
	result, err := s.db.ExecContext(ctx, `
		insert into documents (original_filename, mime_type, size_bytes, sha256, object_key, document_date, description)
		values (?, ?, ?, ?, ?, ?, ?)
	`, input.OriginalFilename, input.MIMEType, input.SizeBytes, input.SHA256, input.ObjectKey, documentDate, input.Description)
	if err != nil {
		return Document{}, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return Document{}, err
	}
	return s.Document(ctx, id)
}

func (s *SQLite) Document(ctx context.Context, id int64) (Document, error) {
	var item Document
	err := scanSQLiteDocument(s.db.QueryRowContext(ctx, sqliteDocumentSelect+` where documents.id = ?`, id), &item)
	if err == sql.ErrNoRows {
		return Document{}, fmt.Errorf("document not found")
	}
	return item, err
}

func (s *SQLite) Documents(ctx context.Context) ([]Document, error) {
	return s.sqliteDocumentsQuery(ctx, sqliteDocumentSelect+` order by documents.created_at desc, documents.id desc`)
}

func (s *SQLite) LinkDocument(ctx context.Context, documentID int64, objectType string, objectID int64) error {
	if !supportedDocumentTarget(objectType) {
		return fmt.Errorf("unsupported object type")
	}
	_, err := s.CreateEntityRelation(ctx, SaveEntityRelation{
		SourceType: ObjectTypeDocument, SourceID: documentID,
		TargetType: objectType, TargetID: objectID, RelationType: RelationTypeEvidenceFor,
	})
	return err
}

func (s *SQLite) DocumentsForObject(ctx context.Context, objectType string, objectID int64) ([]Document, error) {
	if !supportedDocumentTarget(objectType) {
		return nil, fmt.Errorf("unsupported object type")
	}
	return s.sqliteDocumentsQuery(ctx, sqliteDocumentSelect+`
		where exists (
			select 1 from entity_relations
			where source_type = 'document' and source_id = documents.id
			and target_type = ? and target_id = ? and relation_type = 'evidence_for'
		)
		order by documents.created_at desc, documents.id desc
	`, objectType, objectID)
}

func (s *SQLite) UnlinkDocument(ctx context.Context, documentID int64, objectType string, objectID int64) error {
	if !supportedDocumentTarget(objectType) {
		return fmt.Errorf("unsupported object type")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := sqliteValidateEntity(ctx, tx, ObjectTypeDocument, documentID); err != nil {
		return err
	}
	if err := sqliteValidateEntity(ctx, tx, objectType, objectID); err != nil {
		return err
	}
	result, err := tx.ExecContext(ctx, `
		delete from entity_relations
		where source_type = 'document' and source_id = ? and target_type = ? and target_id = ? and relation_type = 'evidence_for'
	`, documentID, objectType, objectID)
	if err != nil {
		return err
	}
	if affected, _ := result.RowsAffected(); affected == 0 {
		return fmt.Errorf("document link not found")
	}
	if validDomainEventOwner(objectType) {
		if err := appendSQLiteDomainEvent(ctx, tx, AppendDomainEvent{
			EntityType: objectType, EntityID: objectID, EventType: "document_unlinked", CurrentDate: true,
			Description: "Document ontkoppeld", RelatedEntityType: ObjectTypeDocument, RelatedEntityID: documentID,
		}); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *SQLite) UpdateDocumentTranslation(ctx context.Context, id int64, status string, text string) error {
	result, err := s.db.ExecContext(ctx, `update documents set translation_status = ?, translation_text = ? where id = ?`, status, text, id)
	if err != nil {
		return err
	}
	if affected, _ := result.RowsAffected(); affected == 0 {
		return fmt.Errorf("document not found")
	}
	_, err = s.db.ExecContext(ctx, `
		insert into document_analyses (document_id, analysis_type, status, language, content_text)
		values (?, 'translation', ?, 'nl', ?)
		on conflict(document_id, analysis_type) do update set status = excluded.status, language = excluded.language, content_text = excluded.content_text, updated_at = current_timestamp
	`, id, status, text)
	if err != nil {
		return err
	}
	return nil
}

const sqliteDocumentSelect = `
	select documents.id, documents.original_filename, documents.mime_type, documents.size_bytes, documents.sha256, documents.object_key,
		coalesce(documents.document_date, ''), coalesce(documents.description, ''),
		coalesce(document_analyses.content_text, documents.translation_text, ''), coalesce(document_analyses.status, documents.translation_status), documents.created_at
	from documents
	left join document_analyses on document_analyses.document_id = documents.id and document_analyses.analysis_type = 'translation'`

func scanSQLiteDocument(scanner interface{ Scan(...any) error }, item *Document) error {
	return scanner.Scan(&item.ID, &item.OriginalFilename, &item.MIMEType, &item.SizeBytes, &item.SHA256, &item.ObjectKey,
		&item.DocumentDate, &item.Description, &item.TranslationText, &item.TranslationStatus, &item.CreatedAt)
}

func (s *SQLite) sqliteDocumentsQuery(ctx context.Context, query string, args ...any) ([]Document, error) {
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []Document{}
	for rows.Next() {
		var item Document
		if err := scanSQLiteDocument(rows, &item); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}
