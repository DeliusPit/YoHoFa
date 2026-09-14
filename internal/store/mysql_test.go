package store

import (
	"context"
	"database/sql"
	"os"
	"strings"
	"testing"
	"time"

	drivermysql "github.com/go-sql-driver/mysql"
)

func TestNormalizeMySQLDSN(t *testing.T) {
	got, err := normalizeMySQLDSN("user:secret@tcp(localhost:3306)/geldzaken?timeout=3s")
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := drivermysql.ParseDSN(got)
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.ParseTime || cfg.Loc != time.UTC || !cfg.ClientFoundRows || cfg.Collation != "utf8mb4_0900_as_cs" {
		t.Fatalf("DSN was not normalized: parseTime=%v loc=%v foundRows=%v collation=%q", cfg.ParseTime, cfg.Loc, cfg.ClientFoundRows, cfg.Collation)
	}
	if cfg.Params["time_zone"] != "'+00:00'" || !strings.Contains(cfg.Params["sql_mode"], "STRICT_TRANS_TABLES") || !strings.Contains(cfg.Params["sql_mode"], "NO_ZERO_DATE") || !strings.Contains(cfg.Params["sql_mode"], "NO_ZERO_IN_DATE") {
		t.Fatalf("missing session parameters: %#v", cfg.Params)
	}
	if strings.Contains(got, "secret ") {
		t.Fatal("unexpected DSN mutation")
	}
}

func TestNormalizeMySQLDSNRejectsInvalidInput(t *testing.T) {
	for _, dsn := range []string{"%", "user@tcp(localhost)/"} {
		if _, err := normalizeMySQLDSN(dsn); err == nil {
			t.Fatalf("expected %q to be rejected", dsn)
		}
	}
}

func TestMySQLTransactionWhere(t *testing.T) {
	query, args := mysqlTransactionWhere(TransactionFilter{AccountID: 2, Search: " rent ", HiddenOnly: true})
	if strings.Count(query, "?") != len(args) {
		t.Fatalf("placeholder mismatch: %d placeholders, %d args", strings.Count(query, "?"), len(args))
	}
	if args[10] != "rent" || args[16] != true {
		t.Fatalf("unexpected normalized filter args: %#v", args)
	}
}

func TestMySQLCanonicalDates(t *testing.T) {
	for _, value := range []string{"", "0000-00-00", "2026-00-17", "2026-02-30", "2026-8-17", " 2026-08-17"} {
		if _, err := mysqlCanonicalDate(value, false); err == nil {
			t.Errorf("expected required date %q to be rejected", value)
		}
	}
	for _, value := range []string{"", "2024-02-29", "2026-08-17"} {
		if _, err := mysqlCanonicalDate(value, true); err != nil {
			t.Errorf("expected optional date %q to be accepted: %v", value, err)
		}
	}
	if err := mysqlValidateTransactionFilter(TransactionFilter{From: "2026-08-18", To: "2026-08-17"}); err == nil {
		t.Fatal("expected reversed filter dates to be rejected")
	}
}

func TestMySQLSchemaDeclarations(t *testing.T) {
	joined := ""
	versions := map[string]bool{}
	for _, migration := range mysqlMigrations {
		if migration.version == "" || versions[migration.version] {
			t.Fatalf("invalid migration version %q", migration.version)
		}
		versions[migration.version] = true
		joined += strings.Join(migration.statements, "\n")
	}
	for _, table := range []string{"households", "accounts", "main_categories", "categories", "transactions", "documents", "assets", "asset_valuations", "liabilities", "cases", "entity_relations", "domain_events"} {
		if !strings.Contains(joined, "create table if not exists "+table) {
			t.Errorf("schema does not declare %s", table)
		}
	}
	for _, required := range []string{"engine=InnoDB", "collate=utf8mb4_0900_as_cs", "fingerprint_unique", "valid_from_key", "creation_key"} {
		if !strings.Contains(joined, required) {
			t.Errorf("schema does not contain %q", required)
		}
	}
	if strings.Contains(joined, "insert ignore") {
		t.Fatal("schema uses broad INSERT IGNORE")
	}
}

func TestMySQLRepositoryLifecycle(t *testing.T) {
	ctx := context.Background()
	db := openMySQLTestStore(t, ctx)
	defer db.Close()

	if err := db.ensureSchema(ctx); err != nil {
		t.Fatal(err)
	}
	mainCategory, err := db.CreateMainCategory(ctx, CreateMainCategory{Name: "MySQL contract"})
	if err != nil {
		t.Fatal(err)
	}
	category, err := db.CreateCategory(ctx, CreateCategory{MainID: mainCategory.ID, Name: "Contract", Kind: "expense"})
	if err != nil {
		t.Fatal(err)
	}
	account, err := db.CreateAccount(ctx, SaveAccount{Name: "Contract account", Type: "checking"})
	if err != nil {
		t.Fatal(err)
	}
	imported, err := db.ImportTransactions(ctx, account.ID, []ImportedTransaction{{BookedOn: "2026-08-17", Description: "Contract", AmountCents: -100}, {BookedOn: "2026-08-17", Description: "Contract", AmountCents: -100}})
	if err != nil || imported.Imported != 2 {
		t.Fatalf("initial import: %+v, %v", imported, err)
	}
	duplicate, err := db.ImportTransactions(ctx, account.ID, []ImportedTransaction{{BookedOn: "2026-08-17", Description: "Contract", AmountCents: -100}})
	if err != nil || duplicate.Duplicates != 1 {
		t.Fatalf("duplicate import: %+v, %v", duplicate, err)
	}
	transactions, err := db.Transactions(ctx, TransactionFilter{AccountID: account.ID})
	if err != nil || len(transactions) != 2 {
		t.Fatalf("transactions: %d, %v", len(transactions), err)
	}
	if _, err := db.CategorizeTransaction(ctx, transactions[0].ID, CategorizeTransaction{CategoryID: category.ID}); err != nil {
		t.Fatal(err)
	}
	if _, err := db.TransactionSummary(ctx, TransactionFilter{AccountID: account.ID}); err != nil {
		t.Fatal(err)
	}
	if _, err := db.CategoryMonthlyTotals(ctx, CategoryReportFilter{CategoryID: category.ID}); err != nil {
		t.Fatal(err)
	}

	asset, err := db.CreateAsset(ctx, SaveAsset{AssetType: "other", Name: "Contract asset"})
	if err != nil {
		t.Fatal(err)
	}
	valuation, err := db.CreateAssetValuation(ctx, asset.ID, SaveAssetValuation{ValuationDate: "2026-08-17", ValueCents: 500, ValuationType: "estimate"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.UpdateAssetValuation(ctx, asset.ID, valuation.ID, SaveAssetValuation{ValuationDate: "2026-08-18", ValueCents: 600, ValuationType: "market"}); err != nil {
		t.Fatal(err)
	}
	liability, err := db.CreateLiability(ctx, SaveLiability{LiabilityType: "loan", Name: "Contract liability"})
	if err != nil {
		t.Fatal(err)
	}
	item, err := db.CreateCase(ctx, SaveCase{CaseType: "contract", Title: "Contract case"})
	if err != nil {
		t.Fatal(err)
	}
	document, err := db.CreateDocument(ctx, CreateDocument{OriginalFilename: "contract.pdf", MIMEType: "application/pdf", SizeBytes: 1, SHA256: strings.Repeat("a", 64), ObjectKey: "contract-key", DocumentDate: "2026-08-17"})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.LinkDocument(ctx, document.ID, ObjectTypeAsset, asset.ID); err != nil {
		t.Fatal(err)
	}
	if err := db.LinkAssetTransaction(ctx, asset.ID, transactions[0].ID); err != nil {
		t.Fatal(err)
	}
	relation, err := db.CreateEntityRelation(ctx, SaveEntityRelation{SourceType: ObjectTypeLiability, SourceID: liability.ID, TargetType: ObjectTypeAsset, TargetID: asset.ID, RelationType: RelationTypeFinancedBy})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.CreateEntityRelation(ctx, SaveEntityRelation{SourceType: ObjectTypeLiability, SourceID: liability.ID, TargetType: ObjectTypeAsset, TargetID: asset.ID, RelationType: RelationTypeFinancedBy}); err != nil {
		t.Fatal("duplicate relation was not idempotent:", err)
	}
	if _, err := db.CreateEntityRelation(ctx, SaveEntityRelation{SourceType: ObjectTypeCase, SourceID: item.ID, TargetType: ObjectTypeLiability, TargetID: liability.ID, RelationType: RelationTypeRelatesTo}); err != nil {
		t.Fatal(err)
	}
	if _, err := db.RelationsForObject(ctx, ObjectTypeAsset, asset.ID); err != nil {
		t.Fatal(err)
	}
	if events, err := db.DomainEventsForObject(ctx, ObjectTypeAsset, asset.ID); err != nil || len(events) == 0 {
		t.Fatalf("domain events: %d, %v", len(events), err)
	}
	if err := db.DeleteEntityRelation(ctx, relation.ID, ObjectTypeAsset, asset.ID); err != nil {
		t.Fatal(err)
	}
}

func TestMySQLConcurrentStartup(t *testing.T) {
	dsn := mysqlTestDSN(t)
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	resetMySQLTestDatabase(t, ctx, dsn)

	const starters = 8
	errs := make(chan error, starters)
	for range starters {
		go func() {
			store, err := OpenMySQL(ctx, dsn)
			if err == nil {
				store.Close()
			}
			errs <- err
		}()
	}
	for range starters {
		if err := <-errs; err != nil {
			t.Fatal(err)
		}
	}

	store, err := OpenMySQL(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	var households, accounts, mainCategories, categories int
	if err := store.db.QueryRowContext(ctx, `select (select count(*) from households),(select count(*) from accounts),(select count(*) from main_categories),(select count(*) from categories)`).Scan(&households, &accounts, &mainCategories, &categories); err != nil {
		t.Fatal(err)
	}
	if households != 1 || accounts != 1 || mainCategories != len(defaultMainCategories()) || categories != len(defaultCategories()) {
		t.Fatalf("concurrent seed produced unexpected counts: households=%d accounts=%d main=%d categories=%d", households, accounts, mainCategories, categories)
	}
}

func TestMySQLRejectsInvalidDates(t *testing.T) {
	ctx := context.Background()
	store := openMySQLTestStore(t, ctx)
	defer store.Close()

	account, err := store.CreateAccount(ctx, SaveAccount{Name: "Date test", Type: "checking"})
	if err != nil {
		t.Fatal(err)
	}
	input := []ImportedTransaction{{BookedOn: "2026-08-17", Description: "valid", AmountCents: 1}, {BookedOn: "2026-02-30", Description: "invalid", AmountCents: 1}}
	if _, err := store.ImportTransactions(ctx, account.ID, input); err == nil {
		t.Fatal("expected invalid imported date to fail")
	}
	var count int
	if err := store.db.QueryRowContext(ctx, `select count(*) from transactions where account_id=?`, account.ID).Scan(&count); err != nil || count != 0 {
		t.Fatalf("invalid batch was partially imported: count=%d err=%v", count, err)
	}
	if _, err := store.Transactions(ctx, TransactionFilter{From: "0000-00-00"}); err == nil {
		t.Fatal("expected zero filter date to fail")
	}
	if _, err := store.TransactionSummary(ctx, TransactionFilter{To: "2026-00-01"}); err == nil {
		t.Fatal("expected partial-zero summary date to fail")
	}
	if _, err := store.MainCategoryPeriodTotals(ctx, PeriodReportFilter{From: "2026-02-30"}); err == nil {
		t.Fatal("expected invalid report date to fail")
	}
	if _, err := store.db.ExecContext(ctx, `insert into transactions(account_id,booked_on,description,amount_cents) values (?, '0000-00-00', 'zero', 1)`, account.ID); err == nil {
		t.Fatal("session SQL mode accepted a zero date")
	}
}

func TestMySQLDomainMutationRollbacks(t *testing.T) {
	t.Run("update on event failure", func(t *testing.T) {
		ctx := context.Background()
		store := openMySQLTestStore(t, ctx)
		defer store.Close()
		asset, err := store.CreateAsset(ctx, SaveAsset{AssetType: "other", Name: "Before"})
		if err != nil {
			t.Fatal(err)
		}
		installMySQLEventFailureConstraint(t, ctx, store)
		if _, err := store.UpdateAsset(ctx, asset.ID, SaveAsset{AssetType: "other", Name: "After"}); err == nil {
			t.Fatal("expected event failure")
		}
		got, err := store.Asset(ctx, asset.ID)
		if err != nil || got.Name != "Before" {
			t.Fatalf("update was not rolled back: name=%q err=%v", got.Name, err)
		}
	})

	t.Run("valuation delete on event failure", func(t *testing.T) {
		ctx := context.Background()
		store := openMySQLTestStore(t, ctx)
		defer store.Close()
		asset, err := store.CreateAsset(ctx, SaveAsset{AssetType: "other", Name: "Valued"})
		if err != nil {
			t.Fatal(err)
		}
		valuation, err := store.CreateAssetValuation(ctx, asset.ID, SaveAssetValuation{ValuationDate: "2026-08-17", ValueCents: 500, ValuationType: "estimate"})
		if err != nil {
			t.Fatal(err)
		}
		installMySQLEventFailureConstraint(t, ctx, store)
		if err = store.DeleteAssetValuation(ctx, asset.ID, valuation.ID); err == nil {
			t.Fatal("expected event failure")
		}
		valuations, err := store.AssetValuations(ctx, asset.ID)
		if err != nil || len(valuations) != 1 {
			t.Fatalf("valuation delete was not rolled back: count=%d err=%v", len(valuations), err)
		}
		got, err := store.Asset(ctx, asset.ID)
		if err != nil || got.CurrentValueCents != 500 {
			t.Fatalf("asset value update was not rolled back: value=%d err=%v", got.CurrentValueCents, err)
		}
	})

	t.Run("delete on database failure", func(t *testing.T) {
		ctx := context.Background()
		store := openMySQLTestStore(t, ctx)
		defer store.Close()
		asset, err := store.CreateAsset(ctx, SaveAsset{AssetType: "other", Name: "Keep"})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := store.db.ExecContext(ctx, `create table mysql_test_delete_block (asset_id bigint primary key, constraint mysql_test_delete_block_fk foreign key (asset_id) references assets(id)) engine=InnoDB`); err != nil {
			t.Fatal(err)
		}
		if _, err := store.db.ExecContext(ctx, `insert into mysql_test_delete_block(asset_id) values (?)`, asset.ID); err != nil {
			t.Fatal(err)
		}
		if err := store.DeleteAsset(ctx, asset.ID); err == nil {
			t.Fatal("expected delete failure")
		}
		if _, err := store.Asset(ctx, asset.ID); err != nil {
			t.Fatalf("asset delete was not rolled back: %v", err)
		}
		events, err := store.DomainEventsForObject(ctx, ObjectTypeAsset, asset.ID)
		if err != nil || len(events) != 1 {
			t.Fatalf("event cleanup was not rolled back: count=%d err=%v", len(events), err)
		}
	})

	t.Run("unlink on event failure", func(t *testing.T) {
		ctx := context.Background()
		store := openMySQLTestStore(t, ctx)
		defer store.Close()
		asset, err := store.CreateAsset(ctx, SaveAsset{AssetType: "other", Name: "Linked"})
		if err != nil {
			t.Fatal(err)
		}
		account, err := store.CreateAccount(ctx, SaveAccount{Name: "Rollback", Type: "checking"})
		if err != nil {
			t.Fatal(err)
		}
		if _, err = store.ImportTransactions(ctx, account.ID, []ImportedTransaction{{BookedOn: "2026-08-17", Description: "Linked", AmountCents: 1}}); err != nil {
			t.Fatal(err)
		}
		transactions, err := store.Transactions(ctx, TransactionFilter{AccountID: account.ID})
		if err != nil {
			t.Fatal(err)
		}
		if err = store.LinkAssetTransaction(ctx, asset.ID, transactions[0].ID); err != nil {
			t.Fatal(err)
		}
		installMySQLEventFailureConstraint(t, ctx, store)
		if err = store.UnlinkAssetTransaction(ctx, asset.ID, transactions[0].ID); err == nil {
			t.Fatal("expected event failure")
		}
		linked, err := store.AssetTransactions(ctx, asset.ID)
		if err != nil || len(linked) != 1 {
			t.Fatalf("unlink was not rolled back: count=%d err=%v", len(linked), err)
		}
	})
}

func mysqlTestDSN(t *testing.T) string {
	t.Helper()
	dsn := os.Getenv("TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("TEST_MYSQL_DSN is not set")
	}
	cfg, err := drivermysql.ParseDSN(dsn)
	if err != nil || cfg.DBName == "" {
		t.Fatal("TEST_MYSQL_DSN must be valid and select a disposable database")
	}
	return dsn
}

func openMySQLTestStore(t *testing.T, ctx context.Context) *MySQL {
	t.Helper()
	dsn := mysqlTestDSN(t)
	resetMySQLTestDatabase(t, ctx, dsn)
	store, err := OpenMySQL(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	return store
}

func resetMySQLTestDatabase(t *testing.T, ctx context.Context, dsn string) {
	t.Helper()
	normalized, err := normalizeMySQLDSN(dsn)
	if err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("mysql", normalized)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	conn, err := db.Conn(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if _, err = conn.ExecContext(ctx, `set foreign_key_checks=0`); err != nil {
		t.Fatal(err)
	}
	for _, table := range []string{"mysql_test_delete_block", "domain_events", "entity_relations", "asset_valuations", "cases", "liabilities", "assets", "documents", "transactions", "categories", "main_categories", "accounts", "households", "schema_migrations"} {
		if _, err = conn.ExecContext(ctx, "drop table if exists `"+table+"`"); err != nil {
			t.Fatal(err)
		}
	}
	if _, err = conn.ExecContext(ctx, `set foreign_key_checks=1`); err != nil {
		t.Fatal(err)
	}
}

func installMySQLEventFailureConstraint(t *testing.T, ctx context.Context, store *MySQL) {
	t.Helper()
	if _, err := store.db.ExecContext(ctx, `alter table domain_events add constraint mysql_test_fail_events check (event_type not in ('asset_updated','valuation_removed','transaction_unlinked'))`); err != nil {
		t.Fatal(err)
	}
}
