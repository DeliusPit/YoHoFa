package store

import (
	"context"
	"os"
	"strings"
	"testing"
)

func TestNormalizeCaseInput(t *testing.T) {
	input, _, _, err := normalizeCaseInput(SaveCase{
		CaseType:    " tax ",
		Title:       " 2025 return ",
		Description: " supporting records ",
		StartDate:   "2025-01-01",
		EndDate:     "2025-12-31",
	})
	if err != nil {
		t.Fatal(err)
	}
	if input.CaseType != "tax" || input.Title != "2025 return" || input.Description != "supporting records" || input.Status != "active" {
		t.Fatalf("unexpected normalized case: %+v", input)
	}
}

func TestNormalizeCaseInputRejectsInvalidValues(t *testing.T) {
	tests := []SaveCase{
		{Title: "Missing type"},
		{CaseType: "tax"},
		{CaseType: "tax", Title: "Return", Status: "open"},
		{CaseType: "tax", Title: "Return", StartDate: "2025-02-01", EndDate: "2025-01-01"},
		{CaseType: "tax", Title: "Return", StartDate: "01-01-2025"},
	}
	for _, input := range tests {
		if _, _, _, err := normalizeCaseInput(input); err == nil {
			t.Fatalf("expected validation error for %+v", input)
		}
	}
}

func TestNormalizeEntityRelationCanonicalizesCaseOrientations(t *testing.T) {
	tests := []struct {
		name       string
		input      SaveEntityRelation
		targetType string
		targetID   int64
	}{
		{
			name: "document", input: SaveEntityRelation{
				SourceType: ObjectTypeCase, SourceID: 10, TargetType: ObjectTypeDocument, TargetID: 20,
				RelationType: RelationTypeEvidenceFor,
			}, targetType: ObjectTypeCase, targetID: 10,
		},
		{
			name: "transaction", input: SaveEntityRelation{
				SourceType: ObjectTypeTransaction, SourceID: 20, TargetType: ObjectTypeCase, TargetID: 10,
				RelationType: RelationTypeRelatesTo,
			}, targetType: ObjectTypeTransaction, targetID: 20,
		},
		{
			name: "asset", input: SaveEntityRelation{
				SourceType: ObjectTypeAsset, SourceID: 20, TargetType: ObjectTypeCase, TargetID: 10,
				RelationType: RelationTypeRelatesTo,
			}, targetType: ObjectTypeAsset, targetID: 20,
		},
		{
			name: "liability", input: SaveEntityRelation{
				SourceType: ObjectTypeLiability, SourceID: 20, TargetType: ObjectTypeCase, TargetID: 10,
				RelationType: RelationTypeRelatesTo,
			}, targetType: ObjectTypeLiability, targetID: 20,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, _, _, err := normalizeEntityRelation(test.input)
			if err != nil {
				t.Fatal(err)
			}
			if test.name == "document" {
				if got.SourceType != ObjectTypeDocument || got.SourceID != 20 || got.TargetType != test.targetType || got.TargetID != test.targetID {
					t.Fatalf("unexpected canonical relation: %+v", got)
				}
				return
			}
			if got.SourceType != ObjectTypeCase || got.SourceID != 10 || got.TargetType != test.targetType || got.TargetID != test.targetID {
				t.Fatalf("unexpected canonical relation: %+v", got)
			}
		})
	}
}

func TestPostgresCaseLifecycle(t *testing.T) {
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

	item, err := db.CreateCase(ctx, SaveCase{CaseType: "tax", Title: "Test case"})
	if err != nil {
		t.Fatal(err)
	}
	asset, err := db.CreateAsset(ctx, SaveAsset{AssetType: "other", Name: "Case test asset"})
	if err != nil {
		t.Fatal(err)
	}
	relation, err := db.CreateEntityRelation(ctx, SaveEntityRelation{
		SourceType: ObjectTypeAsset, SourceID: asset.ID,
		TargetType: ObjectTypeCase, TargetID: item.ID,
		RelationType: RelationTypeRelatesTo,
	})
	if err != nil {
		t.Fatal(err)
	}
	if relation.SourceType != ObjectTypeCase || relation.SourceLabel != item.Title || relation.TargetType != ObjectTypeAsset {
		t.Fatalf("unexpected stored case relation: %+v", relation)
	}
	if err := db.DeleteCase(ctx, item.ID); err == nil || !strings.Contains(err.Error(), "1 object relations") {
		t.Fatalf("expected linked case delete refusal, got %v", err)
	}
	if err := db.DeleteEntityRelation(ctx, relation.ID, ObjectTypeCase, item.ID); err != nil {
		t.Fatal(err)
	}
	if err := db.DeleteAsset(ctx, asset.ID); err != nil {
		t.Fatal(err)
	}
	if err := db.DeleteCase(ctx, item.ID); err != nil {
		t.Fatal(err)
	}
}
