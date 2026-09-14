package store

import (
	"context"
	"database/sql"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func TestSQLiteImplementsRepositoryInterfaces(t *testing.T) {
	var store *SQLite
	var _ Backend = store
	var _ DocumentRepository = store
	var _ AssetRepository = store
	var _ LiabilityRepository = store
	var _ CaseRepository = store
	var _ EntityRelationRepository = store
	var _ DomainEventRepository = store
}

func TestSQLiteUpgradesOldCoreSchemaWithoutLosingData(t *testing.T) {
	path := filepath.Join(t.TempDir(), "upgrade.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`
		pragma foreign_keys = on;
		create table households (id integer primary key, name text not null, created_at text not null default current_timestamp);
		create table accounts (id integer primary key, household_id integer not null default 1 references households(id), name text not null, iban text, account_type text not null default 'checking', owner_name text, bank_profile text not null default 'generic', currency text not null default 'EUR', opening_balance_cents integer not null default 0, sort_order integer not null default 100, created_at text not null default current_timestamp);
		create table main_categories (id integer primary key, name text not null unique, sort_order integer not null default 100, created_at text not null default current_timestamp);
		create table categories (id integer primary key, main_category_id integer references main_categories(id), name text not null, kind text not null, purpose text not null default 'daily', sort_order integer not null default 100, created_at text not null default current_timestamp, unique(main_category_id, name));
		create table transactions (id integer primary key, account_id integer not null references accounts(id), category_id integer references categories(id), booked_on text not null, description text not null, counterparty text, counterparty_iban text, amount_cents integer not null, external_reference text, import_fingerprint text, created_at text not null default current_timestamp);
		insert into households(id, name) values (1, 'Bewaard huishouden');
		insert into accounts(id, household_id, name, account_type, bank_profile) values (7, 1, 'Bewaarde rekening', 'checking', 'generic');
		insert into main_categories(id, name) values (20, 'Bewaard');
		insert into categories(id, main_category_id, name, kind) values (21, 20, 'Bewaarde categorie', 'expense');
		insert into transactions(id, account_id, category_id, booked_on, description, amount_cents) values (22, 7, 21, '2024-01-02', 'Bewaarde transactie', -123);
	`)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	store, err := OpenSQLite(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	transactions, err := store.Transactions(context.Background(), TransactionFilter{Limit: 200})
	if err != nil {
		t.Fatal(err)
	}
	if len(transactions) != 1 || transactions[0].ID != 22 || transactions[0].Description != "Bewaarde transactie" {
		t.Fatalf("core data was not preserved: %+v", transactions)
	}
	asset, err := store.CreateAsset(context.Background(), SaveAsset{AssetType: "other", Name: "Na upgrade"})
	if err != nil || asset.ID == 0 {
		t.Fatalf("new schema unavailable after upgrade: asset=%+v err=%v", asset, err)
	}
	var foreignKeys int
	if err := store.db.QueryRow(`pragma foreign_keys`).Scan(&foreignKeys); err != nil || foreignKeys != 1 {
		t.Fatalf("foreign_keys=%d err=%v", foreignKeys, err)
	}
	if _, err := store.db.Exec(`insert into transactions(account_id, booked_on, description, amount_cents) values (7, '2024-02-30', 'Ongeldig', -1)`); err == nil {
		t.Fatal("upgraded transaction table accepted an invalid date")
	}
	if _, err := store.db.Exec(`update transactions set hidden = 2 where id = 22`); err == nil {
		t.Fatal("upgraded transaction table accepted a non-boolean hidden value")
	}
	if _, err := store.db.Exec(`insert into accounts(household_id, name, account_type, bank_profile) values (1, 'Ongeldig type', 'invalid', 'generic')`); err == nil {
		t.Fatal("upgraded account table accepted an invalid account type")
	}
	if _, err := store.db.Exec(`insert into categories(main_category_id, name, kind, purpose) values (20, 'Ongeldig doel', 'expense', 'invalid')`); err == nil {
		t.Fatal("upgraded category table accepted an invalid purpose")
	}
	if _, err := store.ImportTransactions(context.Background(), 7, []ImportedTransaction{{BookedOn: "2024-03-01", Description: "Nieuwe geldige transactie", AmountCents: -1}}); err != nil {
		t.Fatalf("upgraded transaction table rejected valid data: %v", err)
	}
	store.Close()
	store, err = OpenSQLite(context.Background(), path)
	if err != nil {
		t.Fatalf("second schema upgrade was not idempotent: %v", err)
	}
	defer store.Close()
}

func TestSQLiteRejectsInvalidImportedDateBeforeDuplicateHandling(t *testing.T) {
	ctx := context.Background()
	store := openTestSQLite(t)
	fingerprint := "invalid-date-duplicate"
	if _, err := store.ImportTransactions(ctx, 1, []ImportedTransaction{{
		BookedOn: "2026-01-01", Description: "Geldig", AmountCents: -1, ImportFingerprint: fingerprint,
	}}); err != nil {
		t.Fatal(err)
	}
	result, err := store.ImportTransactions(ctx, 1, []ImportedTransaction{{
		BookedOn: "2026-02-30", Description: "Ongeldig", AmountCents: -1, ImportFingerprint: fingerprint,
	}})
	if err == nil || err.Error() != "invalid booked date" {
		t.Fatalf("invalid date error=%v", err)
	}
	if result != (ImportResult{}) {
		t.Fatalf("invalid date result=%+v, want zero result", result)
	}
	transactions, err := store.Transactions(ctx, TransactionFilter{Limit: 200})
	if err != nil {
		t.Fatal(err)
	}
	if len(transactions) != 1 {
		t.Fatalf("invalid duplicate changed transactions: %+v", transactions)
	}
}

func TestSQLiteReplacementConnectionsKeepPragmas(t *testing.T) {
	path := filepath.Join(t.TempDir(), "replacement.db")
	store, err := OpenSQLite(context.Background(), path+"?_foreign_keys=off&_busy_timeout=1")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(store.Close)
	store.db.SetMaxIdleConns(0)
	for i := 0; i < 3; i++ {
		var foreignKeys, busyTimeout int
		if err := store.db.QueryRow(`select (select foreign_keys from pragma_foreign_keys), (select timeout from pragma_busy_timeout)`).Scan(&foreignKeys, &busyTimeout); err != nil {
			t.Fatal(err)
		}
		if foreignKeys != 1 || busyTimeout != 5000 {
			t.Fatalf("replacement connection pragmas foreign_keys=%d busy_timeout=%d", foreignKeys, busyTimeout)
		}
	}
	if _, err := store.db.Exec(`insert into accounts(household_id, name) values (999, 'Zonder huishouden')`); err == nil {
		t.Fatal("replacement connection did not enforce foreign keys")
	}
}

func TestSQLiteConcurrentOpenIsSerialized(t *testing.T) {
	path := filepath.Join(t.TempDir(), "concurrent.db")
	const opens = 8
	start := make(chan struct{})
	stores := make(chan *SQLite, opens)
	errors := make(chan error, opens)
	var wait sync.WaitGroup
	for i := 0; i < opens; i++ {
		wait.Add(1)
		go func() {
			defer wait.Done()
			<-start
			store, err := OpenSQLite(context.Background(), path)
			if err != nil {
				errors <- err
				return
			}
			stores <- store
		}()
	}
	close(start)
	wait.Wait()
	close(stores)
	close(errors)
	for err := range errors {
		t.Errorf("concurrent OpenSQLite: %v", err)
	}
	var first *SQLite
	count := 0
	for store := range stores {
		count++
		if first == nil {
			first = store
			continue
		}
		store.Close()
	}
	if first != nil {
		defer first.Close()
	}
	if count != opens {
		t.Fatalf("successful opens=%d, want %d", count, opens)
	}
	var households, accounts int
	if err := first.db.QueryRow(`select (select count(*) from households), (select count(*) from accounts)`).Scan(&households, &accounts); err != nil {
		t.Fatal(err)
	}
	if households != 1 || accounts != 1 {
		t.Fatalf("concurrent initialization produced households=%d accounts=%d", households, accounts)
	}
}

func TestSQLiteFailedInitializationRollsBackSchemaChanges(t *testing.T) {
	path := filepath.Join(t.TempDir(), "failed-upgrade.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`
		create table households (id integer primary key, name text not null);
		insert into households(id, name) values (9, 'Bestaand huishouden');
		create table domain_events (id integer primary key);
	`); err != nil {
		db.Close()
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	if store, err := OpenSQLite(context.Background(), path); err == nil {
		store.Close()
		t.Fatal("initialization with an incompatible table unexpectedly succeeded")
	}
	db, err = sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var householdName string
	if err := db.QueryRow(`select name from households where id = 9`).Scan(&householdName); err != nil || householdName != "Bestaand huishouden" {
		t.Fatalf("existing data changed after failed initialization: name=%q err=%v", householdName, err)
	}
	var createdTables int
	if err := db.QueryRow(`select count(*) from sqlite_master where type = 'table' and name in ('accounts', 'documents', 'assets', 'liabilities', 'cases', 'entity_relations')`).Scan(&createdTables); err != nil {
		t.Fatal(err)
	}
	if createdTables != 0 {
		t.Fatalf("failed initialization left %d new tables behind", createdTables)
	}
}

func TestSQLiteAssetValuationTransactionDocumentLifecycle(t *testing.T) {
	ctx := context.Background()
	store := openTestSQLite(t)
	asset, err := store.CreateAsset(ctx, SaveAsset{AssetType: "vehicle", Name: "Auto", PurchaseValueCents: 1000, Currency: "EUR"})
	if err != nil {
		t.Fatal(err)
	}
	older, err := store.CreateAssetValuation(ctx, asset.ID, SaveAssetValuation{ValuationDate: "2025-01-01", ValueCents: 1200, ValuationType: "estimate"})
	if err != nil {
		t.Fatal(err)
	}
	newer, err := store.CreateAssetValuation(ctx, asset.ID, SaveAssetValuation{ValuationDate: "2026-01-01", ValueCents: 1400, ValuationType: "market"})
	if err != nil {
		t.Fatal(err)
	}
	asset, err = store.Asset(ctx, asset.ID)
	if err != nil || asset.CurrentValueCents != 1400 {
		t.Fatalf("current value=%d err=%v", asset.CurrentValueCents, err)
	}
	if _, err := store.ImportTransactions(ctx, 1, []ImportedTransaction{{BookedOn: "2025-06-01", Description: "Onderhoud", AmountCents: -500, ImportFingerprint: "sqlite-parity"}}); err != nil {
		t.Fatal(err)
	}
	transactions, err := store.Transactions(ctx, TransactionFilter{})
	if err != nil || len(transactions) != 1 {
		t.Fatalf("transactions=%+v err=%v", transactions, err)
	}
	transactionID := transactions[0].ID
	if err := store.LinkAssetTransaction(ctx, asset.ID, transactionID); err != nil {
		t.Fatal(err)
	}
	eventsBeforeDuplicate, err := store.DomainEventsForObject(ctx, ObjectTypeAsset, asset.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.LinkAssetTransaction(ctx, asset.ID, transactionID); err != nil {
		t.Fatal(err)
	}
	eventsAfterDuplicate, err := store.DomainEventsForObject(ctx, ObjectTypeAsset, asset.ID)
	if err != nil || len(eventsAfterDuplicate) != len(eventsBeforeDuplicate) {
		t.Fatalf("duplicate relation added events: before=%d after=%d err=%v", len(eventsBeforeDuplicate), len(eventsAfterDuplicate), err)
	}
	document, err := store.CreateDocument(ctx, CreateDocument{OriginalFilename: "factuur.pdf", MIMEType: "application/pdf", SizeBytes: 12, SHA256: strings.Repeat("a", 64), ObjectKey: "docs/factuur", DocumentDate: "2025-05-31"})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.LinkDocument(ctx, document.ID, ObjectTypeTransaction, transactionID); err != nil {
		t.Fatal(err)
	}
	if err := store.LinkDocument(ctx, document.ID, ObjectTypeAsset, asset.ID); err != nil {
		t.Fatal(err)
	}
	transactions, err = store.Transactions(ctx, TransactionFilter{})
	if err != nil || transactions[0].DocumentCount != 1 {
		t.Fatalf("transaction document count=%d err=%v", transactions[0].DocumentCount, err)
	}
	linkedTransactions, err := store.AssetTransactions(ctx, asset.ID)
	if err != nil || len(linkedTransactions) != 1 || linkedTransactions[0].DocumentCount != 1 {
		t.Fatalf("linked transactions=%+v err=%v", linkedTransactions, err)
	}
	if err := store.DeleteAsset(ctx, asset.ID); err == nil || !strings.Contains(err.Error(), "1 document links and 1 object relations") {
		t.Fatalf("asset deletion guard error=%v", err)
	}
	events, err := store.DomainEventsForObject(ctx, ObjectTypeAsset, asset.ID)
	if err != nil {
		t.Fatal(err)
	}
	for i := 1; i < len(events); i++ {
		if events[i-1].EventDate < events[i].EventDate {
			t.Fatalf("events not newest first: %+v", events)
		}
	}
	if err := store.UnlinkDocument(ctx, document.ID, ObjectTypeAsset, asset.ID); err != nil {
		t.Fatal(err)
	}
	if err := store.UnlinkAssetTransaction(ctx, asset.ID, transactionID); err != nil {
		t.Fatal(err)
	}
	if err := store.DeleteAssetValuation(ctx, asset.ID, newer.ID); err != nil {
		t.Fatal(err)
	}
	asset, err = store.Asset(ctx, asset.ID)
	if err != nil || asset.CurrentValueCents != older.ValueCents {
		t.Fatalf("current value after delete=%d err=%v", asset.CurrentValueCents, err)
	}
	if err := store.DeleteAsset(ctx, asset.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := store.DomainEventsForObject(ctx, ObjectTypeAsset, asset.ID); err == nil || err.Error() != "asset not found" {
		t.Fatalf("events for deleted asset error=%v", err)
	}
}

func TestSQLiteLiabilityCaseLifecycleAndEventCleanup(t *testing.T) {
	ctx := context.Background()
	store := openTestSQLite(t)
	liability, err := store.CreateLiability(ctx, SaveLiability{LiabilityType: "loan", Name: "Lening", OriginalAmountCents: 5000, CurrentBalanceCents: 5000})
	if err != nil {
		t.Fatal(err)
	}
	caseItem, err := store.CreateCase(ctx, SaveCase{CaseType: "verbouwing", Title: "Keuken", StartDate: "2026-01-01"})
	if err != nil {
		t.Fatal(err)
	}
	relation, err := store.CreateEntityRelation(ctx, SaveEntityRelation{SourceType: ObjectTypeLiability, SourceID: liability.ID, TargetType: ObjectTypeCase, TargetID: caseItem.ID, RelationType: RelationTypeRelatesTo})
	if err != nil {
		t.Fatal(err)
	}
	if relation.SourceType != ObjectTypeCase || relation.TargetType != ObjectTypeLiability || relation.SourceLabel != "Keuken" || relation.TargetLabel != "Lening" {
		t.Fatalf("relation was not normalized/labeled: %+v", relation)
	}
	if err := store.DeleteLiability(ctx, liability.ID); err == nil {
		t.Fatal("liability with relation was deleted")
	}
	if err := store.DeleteEntityRelation(ctx, relation.ID, ObjectTypeCase, caseItem.ID); err != nil {
		t.Fatal(err)
	}
	liability, err = store.UpdateLiability(ctx, liability.ID, SaveLiability{LiabilityType: "loan", Name: "Lening", OriginalAmountCents: 5000, Status: "repaid"})
	if err != nil || liability.Status != "repaid" {
		t.Fatalf("liability=%+v err=%v", liability, err)
	}
	caseItem, err = store.UpdateCase(ctx, caseItem.ID, SaveCase{CaseType: "verbouwing", Title: "Keuken", Status: "completed"})
	if err != nil || caseItem.Status != "completed" {
		t.Fatalf("case=%+v err=%v", caseItem, err)
	}
	if err := store.DeleteLiability(ctx, liability.ID); err != nil {
		t.Fatal(err)
	}
	caseEvents, err := store.DomainEventsForObject(ctx, ObjectTypeCase, caseItem.ID)
	if err != nil {
		t.Fatal(err)
	}
	foundCleanedRelationEvent := false
	for _, event := range caseEvents {
		if event.EventType == "relation_unlinked" && event.RelatedEntityType == nil && event.RelatedEntityID == nil {
			foundCleanedRelationEvent = true
		}
	}
	if !foundCleanedRelationEvent {
		t.Fatalf("related liability was not cleaned from case events: %+v", caseEvents)
	}
	if err := store.DeleteCase(ctx, caseItem.ID); err != nil {
		t.Fatal(err)
	}
}
