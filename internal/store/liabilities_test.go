package store

import (
	"context"
	"os"
	"strings"
	"testing"
)

func TestNormalizeLiabilityInput(t *testing.T) {
	input, _, _, err := normalizeLiabilityInput(SaveLiability{
		LiabilityType: " mortgage ",
		Name:          " Home loan ",
		Currency:      " eur ",
		StartDate:     "2025-01-01",
		EndDate:       "2035-01-01",
	})
	if err != nil {
		t.Fatal(err)
	}
	if input.Name != "Home loan" || input.Status != "active" || input.Currency != "EUR" {
		t.Fatalf("unexpected normalized liability: %+v", input)
	}
}

func TestNormalizeLiabilityInputRejectsInvalidValues(t *testing.T) {
	tests := []SaveLiability{
		{LiabilityType: "unknown", Name: "Debt"},
		{LiabilityType: "loan", Name: "Debt", CurrentBalanceCents: -1},
		{LiabilityType: "loan", Name: "Debt", Currency: "EU"},
		{LiabilityType: "loan", Name: "Debt", StartDate: "2026-02-01", EndDate: "2026-01-01"},
	}
	for _, input := range tests {
		if _, _, _, err := normalizeLiabilityInput(input); err == nil {
			t.Fatalf("expected validation error for %+v", input)
		}
	}
}

func TestNormalizeEntityRelationCanonicalizesReverseOrientations(t *testing.T) {
	evidence, _, _, err := normalizeEntityRelation(SaveEntityRelation{
		SourceType: ObjectTypeLiability, SourceID: 10,
		TargetType: ObjectTypeDocument, TargetID: 20,
		RelationType: RelationTypeEvidenceFor,
	})
	if err != nil {
		t.Fatal(err)
	}
	if evidence.SourceType != ObjectTypeDocument || evidence.SourceID != 20 || evidence.TargetType != ObjectTypeLiability || evidence.TargetID != 10 {
		t.Fatalf("unexpected evidence orientation: %+v", evidence)
	}

	financing, _, _, err := normalizeEntityRelation(SaveEntityRelation{
		SourceType: ObjectTypeLiability, SourceID: 30,
		TargetType: ObjectTypeAsset, TargetID: 40,
		RelationType: RelationTypeFinancedBy,
	})
	if err != nil {
		t.Fatal(err)
	}
	if financing.SourceType != ObjectTypeAsset || financing.SourceID != 40 || financing.TargetType != ObjectTypeLiability || financing.TargetID != 30 {
		t.Fatalf("unexpected financing orientation: %+v", financing)
	}

	transaction, _, _, err := normalizeEntityRelation(SaveEntityRelation{
		SourceType: ObjectTypeTransaction, SourceID: 50,
		TargetType: ObjectTypeLiability, TargetID: 60,
		RelationType: RelationTypeRelatesTo,
	})
	if err != nil {
		t.Fatal(err)
	}
	if transaction.SourceType != ObjectTypeLiability || transaction.SourceID != 60 || transaction.TargetType != ObjectTypeTransaction || transaction.TargetID != 50 {
		t.Fatalf("unexpected transaction orientation: %+v", transaction)
	}
}

func TestPostgresLiabilityLifecycle(t *testing.T) {
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

	liability, err := db.CreateLiability(ctx, SaveLiability{LiabilityType: "mortgage", Name: "Test mortgage"})
	if err != nil {
		t.Fatal(err)
	}
	asset, err := db.CreateAsset(ctx, SaveAsset{AssetType: "real_estate", Name: "Test property"})
	if err != nil {
		t.Fatal(err)
	}
	relation, err := db.CreateEntityRelation(ctx, SaveEntityRelation{
		SourceType: ObjectTypeLiability, SourceID: liability.ID,
		TargetType: ObjectTypeAsset, TargetID: asset.ID,
		RelationType: RelationTypeFinancedBy,
	})
	if err != nil {
		t.Fatal(err)
	}
	if relation.SourceType != ObjectTypeAsset || relation.TargetType != ObjectTypeLiability {
		t.Fatalf("unexpected stored relation orientation: %+v", relation)
	}
	if err := db.DeleteLiability(ctx, liability.ID); err == nil || !strings.Contains(err.Error(), "1 object relations") {
		t.Fatalf("expected linked liability delete refusal, got %v", err)
	}
	if err := db.DeleteEntityRelation(ctx, relation.ID, ObjectTypeLiability, liability.ID); err != nil {
		t.Fatal(err)
	}
	if err := db.DeleteAsset(ctx, asset.ID); err != nil {
		t.Fatal(err)
	}
	if err := db.DeleteLiability(ctx, liability.ID); err != nil {
		t.Fatal(err)
	}
}
