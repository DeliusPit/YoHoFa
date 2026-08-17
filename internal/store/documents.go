package store

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

const ObjectTypeTransaction = "transaction"

type Document struct {
	ID               int64  `json:"id"`
	OriginalFilename string `json:"originalFilename"`
	MIMEType         string `json:"mimeType"`
	SizeBytes        int64  `json:"sizeBytes"`
	SHA256           string `json:"sha256"`
	ObjectKey        string `json:"-"`
	DocumentDate     string `json:"documentDate,omitempty"`
	Description      string `json:"description,omitempty"`
	CreatedAt        string `json:"createdAt"`
}

type CreateDocument struct {
	OriginalFilename string
	MIMEType         string
	SizeBytes        int64
	SHA256           string
	ObjectKey        string
	DocumentDate     string
	Description      string
}

type DocumentRepository interface {
	CreateDocument(context.Context, CreateDocument) (Document, error)
	Document(context.Context, int64) (Document, error)
	Documents(context.Context) ([]Document, error)
	LinkDocument(context.Context, int64, string, int64) error
	DocumentsForObject(context.Context, string, int64) ([]Document, error)
	UnlinkDocument(context.Context, int64, string, int64) error
}

func (s *Postgres) CreateDocument(ctx context.Context, input CreateDocument) (Document, error) {
	var item Document
	var documentDate *time.Time
	if input.DocumentDate != "" {
		parsed, err := time.Parse("2006-01-02", input.DocumentDate)
		if err != nil {
			return Document{}, fmt.Errorf("invalid document date")
		}
		documentDate = &parsed
	}
	err := s.pool.QueryRow(ctx, `
		insert into documents (original_filename, mime_type, size_bytes, sha256, object_key, document_date, description)
		values ($1, $2, $3, $4, $5, $6, $7)
		returning id, original_filename, mime_type, size_bytes, sha256, object_key,
			coalesce(to_char(document_date, 'YYYY-MM-DD'), ''), coalesce(description, ''), created_at::text
	`, input.OriginalFilename, input.MIMEType, input.SizeBytes, input.SHA256, input.ObjectKey, documentDate, input.Description).Scan(
		&item.ID, &item.OriginalFilename, &item.MIMEType, &item.SizeBytes, &item.SHA256, &item.ObjectKey,
		&item.DocumentDate, &item.Description, &item.CreatedAt,
	)
	return item, err
}

func (s *Postgres) Document(ctx context.Context, id int64) (Document, error) {
	var item Document
	err := s.pool.QueryRow(ctx, `
		select id, original_filename, mime_type, size_bytes, sha256, object_key,
			coalesce(to_char(document_date, 'YYYY-MM-DD'), ''), coalesce(description, ''), created_at::text
		from documents where id = $1
	`, id).Scan(&item.ID, &item.OriginalFilename, &item.MIMEType, &item.SizeBytes, &item.SHA256, &item.ObjectKey, &item.DocumentDate, &item.Description, &item.CreatedAt)
	if err == pgx.ErrNoRows {
		return Document{}, fmt.Errorf("document not found")
	}
	return item, err
}

func (s *Postgres) Documents(ctx context.Context) ([]Document, error) {
	return s.documentsQuery(ctx, `
		select id, original_filename, mime_type, size_bytes, sha256, object_key,
			coalesce(to_char(document_date, 'YYYY-MM-DD'), ''), coalesce(description, ''), created_at::text
		from documents order by created_at desc, id desc
	`)
}

func (s *Postgres) LinkDocument(ctx context.Context, documentID int64, objectType string, objectID int64) error {
	if !supportedDocumentTarget(objectType) {
		return fmt.Errorf("unsupported object type")
	}
	_, err := s.CreateEntityRelation(ctx, SaveEntityRelation{
		SourceType: ObjectTypeDocument, SourceID: documentID,
		TargetType: objectType, TargetID: objectID,
		RelationType: RelationTypeEvidenceFor,
	})
	return err
}

func (s *Postgres) DocumentsForObject(ctx context.Context, objectType string, objectID int64) ([]Document, error) {
	if !supportedDocumentTarget(objectType) {
		return nil, fmt.Errorf("unsupported object type")
	}
	return s.documentsQuery(ctx, `
		select documents.id, documents.original_filename, documents.mime_type, documents.size_bytes, documents.sha256, documents.object_key,
			coalesce(to_char(documents.document_date, 'YYYY-MM-DD'), ''), coalesce(documents.description, ''), documents.created_at::text
		from documents
		where exists (
			select 1 from entity_relations
			where source_type = 'document' and source_id = documents.id
			and target_type = $1 and target_id = $2 and relation_type = 'evidence_for'
		)
		order by documents.created_at desc, documents.id desc
	`, objectType, objectID)
}

func (s *Postgres) UnlinkDocument(ctx context.Context, documentID int64, objectType string, objectID int64) error {
	if !supportedDocumentTarget(objectType) {
		return fmt.Errorf("unsupported object type")
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	firstType, firstID := ObjectTypeDocument, documentID
	secondType, secondID := objectType, objectID
	if secondType < firstType || (secondType == firstType && secondID < firstID) {
		firstType, secondType = secondType, firstType
		firstID, secondID = secondID, firstID
	}
	if err := validateAndLockEntity(ctx, tx, firstType, firstID); err != nil {
		return err
	}
	if err := validateAndLockEntity(ctx, tx, secondType, secondID); err != nil {
		return err
	}
	result, err := tx.Exec(ctx, `
		delete from entity_relations
		where source_type = 'document' and source_id = $1 and target_type = $2 and target_id = $3 and relation_type = 'evidence_for'
	`, documentID, objectType, objectID)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return fmt.Errorf("document link not found")
	}
	if validDomainEventOwner(objectType) {
		if err := appendDomainEvent(ctx, tx, AppendDomainEvent{
			EntityType: objectType, EntityID: objectID, EventType: "document_unlinked", CurrentDate: true,
			Description: "Document ontkoppeld", RelatedEntityType: ObjectTypeDocument, RelatedEntityID: documentID,
		}); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func supportedDocumentTarget(objectType string) bool {
	return objectType == ObjectTypeTransaction || objectType == ObjectTypeAsset || objectType == ObjectTypeLiability || objectType == ObjectTypeCase
}

func (s *Postgres) documentsQuery(ctx context.Context, query string, args ...any) ([]Document, error) {
	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := []Document{}
	for rows.Next() {
		var item Document
		if err := rows.Scan(&item.ID, &item.OriginalFilename, &item.MIMEType, &item.SizeBytes, &item.SHA256, &item.ObjectKey, &item.DocumentDate, &item.Description, &item.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}
