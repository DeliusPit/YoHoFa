package store

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"
)

func TestCoreEmptySchemaAndReopen(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "new", "hohoho.db")
	s, err := OpenCore(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	rows, err := s.db.QueryContext(ctx, "SELECT name FROM sqlite_master WHERE type = 'table'")
	if err != nil {
		t.Fatal(err)
	}
	var tables []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatal(err)
		}
		tables = append(tables, name)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	rows.Close()
	if len(tables) != 9 {
		t.Fatalf("want exactly nine tables, got %v", tables)
	}
	for _, table := range tables {
		var count int
		if err := s.db.QueryRowContext(ctx, "SELECT count(*) FROM "+table).Scan(&count); err != nil || count != 0 {
			t.Fatalf("table %s must be empty: count=%d err=%v", table, count, err)
		}
	}
	var version, foreignKeys int
	var journal string
	if err := s.db.QueryRowContext(ctx, "PRAGMA user_version").Scan(&version); err != nil || version != CoreSchemaVersion {
		t.Fatalf("schema version: %d, %v", version, err)
	}
	if err := s.db.QueryRowContext(ctx, "PRAGMA journal_mode").Scan(&journal); err != nil || journal != "wal" {
		t.Fatalf("journal mode: %s, %v", journal, err)
	}
	// Force new physical connections: foreign keys must not only be set on startup.
	s.db.SetMaxIdleConns(0)
	if err := s.db.QueryRowContext(ctx, "PRAGMA foreign_keys").Scan(&foreignKeys); err != nil || foreignKeys != 1 {
		t.Fatalf("foreign keys disabled on replacement connection: %d, %v", foreignKeys, err)
	}
	if _, err := s.db.ExecContext(ctx, "INSERT INTO object_values (financial_object_id,value_date,value_cents) VALUES (999,'2026-10-01',1)"); err == nil {
		t.Fatal("foreign key must reject missing object")
	}
	if _, err := s.db.ExecContext(ctx, "INSERT INTO bank_import_profiles(name,source_format,configuration) VALUES ('Bad','csv','not-json')"); err == nil {
		t.Fatal("invalid JSON must be rejected")
	}
	object, err := s.CreateFinancialObject(ctx, SaveFinancialObject{ObjectType: "asset", Name: "Woning"})
	if err != nil {
		t.Fatal(err)
	}
	s.Close()
	s, err = OpenCore(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	objects, err := s.FinancialObjects(ctx, "")
	if err != nil || len(objects) != 1 || objects[0].ID != object.ID {
		t.Fatalf("reopen must preserve data without seeding: %v, %v", objects, err)
	}
}

func TestCoreRejectsLegacyAndFutureDatabases(t *testing.T) {
	for _, test := range []struct{ name, setup string }{
		{"legacy", "CREATE TABLE households (id INTEGER PRIMARY KEY); INSERT INTO households VALUES (1)"},
		{"future", "PRAGMA user_version = 999"},
		{"wrong-version-one", "CREATE TABLE accounts (id INTEGER PRIMARY KEY); PRAGMA user_version = 1"},
	} {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "source.db")
			db, err := sql.Open("sqlite", path)
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			if _, err := db.Exec(test.setup); err != nil {
				t.Fatal(err)
			}
			if s, err := OpenCore(context.Background(), path); err == nil {
				s.Close()
				t.Fatal("unrecognized schema must not be initialized")
			}
			var count int
			if err := db.QueryRow("SELECT count(*) FROM sqlite_master WHERE name = 'financial_objects'").Scan(&count); err != nil || count != 0 {
				t.Fatalf("source was modified: %d, %v", count, err)
			}
		})
	}
}

func TestCoreObjectsValuesAndRelations(t *testing.T) {
	ctx := context.Background()
	s, err := OpenCore(ctx, filepath.Join(t.TempDir(), "core.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	for _, input := range []SaveFinancialObject{{ObjectType: "income", Name: "Wrong type"}, {ObjectType: "asset", Name: " "}} {
		if _, err := s.CreateFinancialObject(ctx, input); !errors.Is(err, ErrCoreInvalid) {
			t.Fatalf("invalid object accepted: %v", err)
		}
	}
	var objects []FinancialObject
	for _, kind := range []string{"asset", "debt", "expense", "reserve"} {
		object, err := s.CreateFinancialObject(ctx, SaveFinancialObject{ObjectType: kind, Name: " " + kind + " "})
		if err != nil || object.Name != kind {
			t.Fatalf("create %s: %v, %v", kind, object, err)
		}
		objects = append(objects, object)
	}
	asset, debt := objects[0], objects[1]
	filtered, err := s.FinancialObjects(ctx, "debt")
	if err != nil || len(filtered) != 1 || filtered[0].ID != debt.ID {
		t.Fatalf("type filter: %v, %v", filtered, err)
	}
	for _, input := range []SaveObjectValue{{ValueDate: "2026-02-30", ValueCents: cents(1)}, {ValueDate: "2026-10-01"}} {
		if _, err := s.CreateObjectValue(ctx, asset.ID, input); !errors.Is(err, ErrCoreInvalid) {
			t.Fatalf("invalid valuation accepted: %v", err)
		}
	}
	for _, input := range []SaveObjectValue{
		{ValueDate: "2026-10-01", ValueCents: cents(5000000000)},
		{ValueDate: "2026-01-01", ValueCents: cents(6000000000)},
		{ValueDate: "2026-10-01", ValueCents: cents(-12345)},
	} {
		if _, err := s.CreateObjectValue(ctx, asset.ID, input); err != nil {
			t.Fatal(err)
		}
	}
	item, err := s.FinancialObject(ctx, asset.ID)
	if err != nil || item.CurrentValue == nil || item.CurrentValue.ValueCents != -12345 {
		t.Fatalf("latest date then ID must determine value: %+v, %v", item, err)
	}
	if err := s.DeleteObjectValue(ctx, debt.ID, item.CurrentValue.ID); !errors.Is(err, ErrCoreNotFound) {
		t.Fatalf("must not delete another object's value: %v", err)
	}
	if err := s.DeleteFinancialObject(ctx, asset.ID); !errors.Is(err, ErrCoreConflict) {
		t.Fatalf("valued object deletion must be blocked: %v", err)
	}
	relation := ObjectRelation{SourceObjectID: debt.ID, TargetObjectID: asset.ID, RelationType: "finances"}
	if _, err := s.CreateObjectRelation(ctx, relation); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateObjectRelation(ctx, relation); !errors.Is(err, ErrCoreConflict) {
		t.Fatalf("duplicate relation accepted: %v", err)
	}
	if _, err := s.CreateObjectRelation(ctx, ObjectRelation{SourceObjectID: asset.ID, TargetObjectID: asset.ID, RelationType: "self"}); !errors.Is(err, ErrCoreInvalid) {
		t.Fatalf("self relation accepted: %v", err)
	}
	if _, err := s.CreateObjectRelation(ctx, ObjectRelation{SourceObjectID: asset.ID, TargetObjectID: 999, RelationType: "free text"}); !errors.Is(err, ErrCoreNotFound) {
		t.Fatalf("missing relation endpoint accepted: %v", err)
	}
	for _, id := range []int64{asset.ID, debt.ID} {
		relations, err := s.ObjectRelations(ctx, id)
		if err != nil || len(relations) != 1 || relations[0] != relation {
			t.Fatalf("incoming and outgoing relations must be visible: %v, %v", relations, err)
		}
	}
	if err := s.DeleteFinancialObject(ctx, debt.ID); !errors.Is(err, ErrCoreConflict) {
		t.Fatalf("related object deletion must be blocked: %v", err)
	}
	if err := s.DeleteObjectRelation(ctx, relation); err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteFinancialObject(ctx, debt.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.FinancialObject(ctx, debt.ID); !errors.Is(err, ErrCoreNotFound) {
		t.Fatalf("deleted object still exists: %v", err)
	}
	values, err := s.ObjectValues(ctx, asset.ID)
	if err != nil || len(values) != 3 {
		t.Fatalf("history must remain intact: %v, %v", values, err)
	}
	if err := s.DeleteObjectValue(ctx, asset.ID, item.CurrentValue.ID); err != nil {
		t.Fatal(err)
	}
	item, err = s.FinancialObject(ctx, asset.ID)
	if err != nil || item.CurrentValue.ValueCents != 5000000000 {
		t.Fatalf("delete must expose previous latest value: %+v, %v", item, err)
	}
}

func cents(value int64) *int64 { return &value }
