package store

import (
	"context"
	"testing"
)

func TestSQLiteMainCategoryManagesCaseLifecycle(t *testing.T) {
	db := openTestSQLite(t)
	ctx := context.Background()
	mainCategories, err := db.MainCategories(ctx)
	if err != nil {
		t.Fatal(err)
	}
	managedCases, err := db.Cases(ctx, CaseFilter{CaseType: CaseTypeMainCategory})
	if err != nil {
		t.Fatal(err)
	}
	if len(managedCases) != len(mainCategories) {
		t.Fatalf("managed cases = %d, main categories = %d", len(managedCases), len(mainCategories))
	}

	created, err := db.CreateMainCategory(ctx, CreateMainCategory{Name: "Huisdieren"})
	if err != nil {
		t.Fatal(err)
	}
	managed := findManagedCase(t, db, created.ID)
	if managed.Title != created.Name || managed.Status != "active" {
		t.Fatalf("unexpected managed case: %+v", managed)
	}
	if _, err := db.UpdateCase(ctx, managed.ID, SaveCase{CaseType: "other", Title: "Los dossier", Status: "active"}); err == nil {
		t.Fatal("direct update of managed case must be rejected")
	}
	if err := db.DeleteCase(ctx, managed.ID); err == nil {
		t.Fatal("direct delete of managed case must be rejected")
	}

	updated, err := db.UpdateMainCategory(ctx, created.ID, UpdateMainCategory{Name: "Dieren"})
	if err != nil {
		t.Fatal(err)
	}
	managed = findManagedCase(t, db, created.ID)
	if managed.Title != updated.Name {
		t.Fatalf("managed case title = %q, want %q", managed.Title, updated.Name)
	}

	if _, err := db.db.ExecContext(ctx, `insert into entity_relations(source_type,source_id,target_type,target_id,relation_type) values ('case',?,'asset',1,'relates_to')`, managed.ID); err != nil {
		t.Fatal(err)
	}
	if err := db.DeleteMainCategory(ctx, created.ID); err == nil {
		t.Fatal("delete with managed case relation must be rejected")
	}
	if _, err := db.db.ExecContext(ctx, `delete from entity_relations where source_type='case' and source_id=?`, managed.ID); err != nil {
		t.Fatal(err)
	}
	if err := db.DeleteMainCategory(ctx, created.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Case(ctx, managed.ID); err == nil {
		t.Fatal("managed case still exists after deleting main category")
	}
}

func findManagedCase(t *testing.T, db *SQLite, mainCategoryID int64) Case {
	t.Helper()
	items, err := db.Cases(context.Background(), CaseFilter{CaseType: CaseTypeMainCategory})
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range items {
		if item.MainCategoryID == mainCategoryID {
			return item
		}
	}
	t.Fatalf("managed case for main category %d not found", mainCategoryID)
	return Case{}
}
