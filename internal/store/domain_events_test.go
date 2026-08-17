package store

import (
	"context"
	"os"
	"testing"
	"time"
)

func TestValidateAppendDomainEvent(t *testing.T) {
	valid := AppendDomainEvent{
		EntityType: ObjectTypeAsset, EntityID: 1, EventType: "asset_updated",
		EventDate: time.Now(), Description: "Bezitting bijgewerkt",
	}
	if err := validateAppendDomainEvent(valid); err != nil {
		t.Fatal(err)
	}

	invalid := []AppendDomainEvent{
		{},
		{EntityType: ObjectTypeTransaction, EntityID: 1, EventType: "updated", EventDate: time.Now(), Description: "Updated"},
		{EntityType: ObjectTypeAsset, EntityID: 1, EventType: "updated", EventDate: time.Now(), Description: "Updated", RelatedEntityType: ObjectTypeDocument},
		{EntityType: ObjectTypeAsset, EntityID: 1, EventType: "updated", EventDate: time.Now(), Description: "Updated", RelatedEntityID: 2},
		{EntityType: ObjectTypeAsset, EntityID: 1, EventType: "updated", EventDate: time.Now(), Description: "Updated", RelatedEntityType: "account", RelatedEntityID: 2},
		{EntityType: ObjectTypeAsset, EntityID: 1, EventType: "updated", EventDate: time.Now(), Description: "Updated", RelatedEntityType: ObjectTypeAsset, RelatedEntityID: 1},
	}
	for _, input := range invalid {
		if err := validateAppendDomainEvent(input); err == nil {
			t.Fatalf("expected validation error for %+v", input)
		}
	}
}

func TestPostgresDomainEventLifecycle(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	ctx := context.Background()
	db, err := OpenPostgres(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	asset, err := db.CreateAsset(ctx, SaveAsset{AssetType: "other", Name: "Domain event asset"})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.ensureSchema(ctx); err != nil {
		t.Fatal(err)
	}
	events := mustDomainEvents(t, db, ObjectTypeAsset, asset.ID)
	if countDomainEvents(events, "asset_created") != 1 {
		t.Fatalf("expected one creation event, got %+v", events)
	}
	if _, err := db.CreateAssetValuation(ctx, asset.ID, SaveAssetValuation{
		ValuationDate: "2026-06-15", ValueCents: 12345, ValuationType: "estimate",
	}); err != nil {
		t.Fatal(err)
	}
	if countDomainEvents(mustDomainEvents(t, db, ObjectTypeAsset, asset.ID), "valuation_added") != 1 {
		t.Fatal("expected valuation_added event")
	}

	var accountID int64
	if err := db.pool.QueryRow(ctx, `select id from accounts order by id limit 1`).Scan(&accountID); err != nil {
		t.Fatal(err)
	}
	var transactionID int64
	if err := db.pool.QueryRow(ctx, `
		insert into transactions (account_id, booked_on, description, amount_cents)
		values ($1, '2026-06-16', 'Domain event transaction', -100) returning id
	`, accountID).Scan(&transactionID); err != nil {
		t.Fatal(err)
	}
	transactionRelation := SaveEntityRelation{
		SourceType: ObjectTypeAsset, SourceID: asset.ID, TargetType: ObjectTypeTransaction, TargetID: transactionID,
		RelationType: RelationTypeRelatesTo,
	}
	if _, err := db.CreateEntityRelation(ctx, transactionRelation); err != nil {
		t.Fatal(err)
	}
	if _, err := db.CreateEntityRelation(ctx, transactionRelation); err != nil {
		t.Fatal(err)
	}
	if countDomainEvents(mustDomainEvents(t, db, ObjectTypeAsset, asset.ID), "transaction_linked") != 1 {
		t.Fatal("duplicate relation emitted a duplicate transaction_linked event")
	}
	if err := db.UnlinkAssetTransaction(ctx, asset.ID, transactionID); err != nil {
		t.Fatal(err)
	}
	if countDomainEvents(mustDomainEvents(t, db, ObjectTypeAsset, asset.ID), "transaction_unlinked") != 1 {
		t.Fatal("expected one transaction_unlinked event")
	}

	var documentID int64
	if err := db.pool.QueryRow(ctx, `
		insert into documents (original_filename, mime_type, size_bytes, sha256, object_key, document_date)
		values ('domain-event.pdf', 'application/pdf', 1, $1, $2, '2026-06-17') returning id
	`, "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "domain-event-"+time.Now().Format("20060102150405.000000000")).Scan(&documentID); err != nil {
		t.Fatal(err)
	}
	if err := db.LinkDocument(ctx, documentID, ObjectTypeAsset, asset.ID); err != nil {
		t.Fatal(err)
	}
	if err := db.LinkDocument(ctx, documentID, ObjectTypeAsset, asset.ID); err != nil {
		t.Fatal(err)
	}
	if err := db.UnlinkDocument(ctx, documentID, ObjectTypeAsset, asset.ID); err != nil {
		t.Fatal(err)
	}
	events = mustDomainEvents(t, db, ObjectTypeAsset, asset.ID)
	if countDomainEvents(events, "document_linked") != 1 || countDomainEvents(events, "document_unlinked") != 1 {
		t.Fatalf("unexpected document event counts: %+v", events)
	}

	liability, err := db.CreateLiability(ctx, SaveLiability{LiabilityType: "loan", Name: "Domain event liability"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.UpdateLiability(ctx, liability.ID, SaveLiability{LiabilityType: "loan", Name: liability.Name, Status: "repaid"}); err != nil {
		t.Fatal(err)
	}
	if countDomainEvents(mustDomainEvents(t, db, ObjectTypeLiability, liability.ID), "liability_repaid") != 1 {
		t.Fatal("expected liability_repaid event")
	}

	item, err := db.CreateCase(ctx, SaveCase{CaseType: "tax", Title: "Domain event case"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.UpdateCase(ctx, item.ID, SaveCase{CaseType: item.CaseType, Title: item.Title, Status: "completed"}); err != nil {
		t.Fatal(err)
	}
	if countDomainEvents(mustDomainEvents(t, db, ObjectTypeCase, item.ID), "case_completed") != 1 {
		t.Fatal("expected case_completed event")
	}
	relation, err := db.CreateEntityRelation(ctx, SaveEntityRelation{
		SourceType: ObjectTypeCase, SourceID: item.ID, TargetType: ObjectTypeAsset, TargetID: asset.ID,
		RelationType: RelationTypeRelatesTo,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.DeleteEntityRelation(ctx, relation.ID, ObjectTypeCase, item.ID); err != nil {
		t.Fatal(err)
	}
	if err := db.DeleteAsset(ctx, asset.ID); err != nil {
		t.Fatal(err)
	}
	for _, event := range mustDomainEvents(t, db, ObjectTypeCase, item.ID) {
		if event.EventType == "relation_unlinked" && (event.RelatedEntityType != nil || event.RelatedEntityID != nil || event.RelatedEntityLabel != nil) {
			t.Fatalf("deleted related owner was not cleared: %+v", event)
		}
	}
	var assetEventCount int
	if err := db.pool.QueryRow(ctx, `select count(*) from domain_events where entity_type = 'asset' and entity_id = $1`, asset.ID).Scan(&assetEventCount); err != nil {
		t.Fatal(err)
	}
	if assetEventCount != 0 {
		t.Fatalf("deleted asset retained %d events", assetEventCount)
	}

	if err := db.DeleteCase(ctx, item.ID); err != nil {
		t.Fatal(err)
	}
	if err := db.DeleteLiability(ctx, liability.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.pool.Exec(ctx, `delete from documents where id = $1`, documentID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.pool.Exec(ctx, `delete from transactions where id = $1`, transactionID); err != nil {
		t.Fatal(err)
	}
}

func mustDomainEvents(t *testing.T, db *Postgres, entityType string, entityID int64) []DomainEvent {
	t.Helper()
	events, err := db.DomainEventsForObject(context.Background(), entityType, entityID)
	if err != nil {
		t.Fatal(err)
	}
	return events
}

func countDomainEvents(events []DomainEvent, eventType string) int {
	count := 0
	for _, event := range events {
		if event.EventType == eventType {
			count++
		}
	}
	return count
}
