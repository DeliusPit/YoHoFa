package store

import (
	"context"
	"path/filepath"
	"testing"
)

func TestDeleteAccountWithTransactionsIsRejected(t *testing.T) {
	store := openTestSQLite(t)
	_, err := store.ImportTransactions(context.Background(), 1, []ImportedTransaction{{
		BookedOn:          "2026-01-01",
		Description:       "Test",
		AmountCents:       -100,
		ImportFingerprint: "delete-account-test",
	}})
	if err != nil {
		t.Fatal(err)
	}

	if err := store.DeleteAccount(context.Background(), 1); err == nil {
		t.Fatal("deleting an account with transactions must be rejected")
	}
}

func TestImportPreservesRepeatedTransactionsAndDeduplicatesReimport(t *testing.T) {
	store := openTestSQLite(t)
	transaction := ImportedTransaction{
		BookedOn:    "2026-01-01",
		Description: "Dezelfde betaling",
		AmountCents: -1000,
	}

	first, err := store.ImportTransactions(context.Background(), 1, []ImportedTransaction{transaction, transaction})
	if err != nil {
		t.Fatal(err)
	}
	if first.Imported != 2 || first.Duplicates != 0 {
		t.Fatalf("first import = %+v, want 2 imported and 0 duplicates", first)
	}

	second, err := store.ImportTransactions(context.Background(), 1, []ImportedTransaction{transaction, transaction})
	if err != nil {
		t.Fatal(err)
	}
	if second.Imported != 0 || second.Duplicates != 2 {
		t.Fatalf("second import = %+v, want 0 imported and 2 duplicates", second)
	}
}

func TestMainCategoryPeriodTotalsDrillsDownToSubcategories(t *testing.T) {
	store := openTestSQLite(t)
	_, err := store.ImportTransactions(context.Background(), 1, []ImportedTransaction{
		{BookedOn: "2026-01-01", Description: "Huur", AmountCents: -10000, ImportFingerprint: "report-rent"},
		{BookedOn: "2026-01-02", Description: "Onderhoud", AmountCents: -2500, ImportFingerprint: "report-maintenance"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.CategorizeTransaction(context.Background(), 1, CategorizeTransaction{CategoryID: 2}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.CategorizeTransaction(context.Background(), 2, CategorizeTransaction{CategoryID: 4}); err != nil {
		t.Fatal(err)
	}

	totals, err := store.MainCategoryPeriodTotals(context.Background(), PeriodReportFilter{MainCategoryID: 2})
	if err != nil {
		t.Fatal(err)
	}
	if len(totals) != 2 {
		t.Fatalf("got %d totals, want 2", len(totals))
	}
	if totals[0].CategoryID == 0 || totals[0].CategoryName == "" || totals[1].CategoryID == 0 || totals[1].CategoryName == "" {
		t.Fatalf("expected subcategory details, got %+v", totals)
	}
}

func TestTransactionYearsReturnsVisibleDistinctYears(t *testing.T) {
	store := openTestSQLite(t)
	_, err := store.ImportTransactions(context.Background(), 1, []ImportedTransaction{
		{BookedOn: "2025-12-31", Description: "Oud", AmountCents: -100, ImportFingerprint: "year-2025"},
		{BookedOn: "2026-01-01", Description: "Nieuw", AmountCents: -100, ImportFingerprint: "year-2026-a"},
		{BookedOn: "2026-02-01", Description: "Nieuw 2", AmountCents: -100, ImportFingerprint: "year-2026-b"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.SetTransactionHidden(context.Background(), 1, true); err != nil {
		t.Fatal(err)
	}

	years, err := store.TransactionYears(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(years) != 1 || years[0] != 2026 {
		t.Fatalf("years = %v, want [2026]", years)
	}
}

func TestDeleteCategoryInUseIsRejected(t *testing.T) {
	store := openTestSQLite(t)
	_, err := store.ImportTransactions(context.Background(), 1, []ImportedTransaction{{
		BookedOn:          "2026-01-01",
		Description:       "Test",
		AmountCents:       -100,
		ImportFingerprint: "delete-category-test",
	}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.CategorizeTransaction(context.Background(), 1, CategorizeTransaction{CategoryID: 1}); err != nil {
		t.Fatal(err)
	}

	if err := store.DeleteCategory(context.Background(), 1); err == nil {
		t.Fatal("deleting a category used by transactions must be rejected")
	}
}

func TestDeleteMainCategoryWithSubcategoriesIsRejected(t *testing.T) {
	store := openTestSQLite(t)
	if err := store.DeleteMainCategory(context.Background(), 2); err == nil {
		t.Fatal("deleting a main category with subcategories must be rejected")
	}
}

func openTestSQLite(t *testing.T) *SQLite {
	t.Helper()
	store, err := OpenSQLite(context.Background(), filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(store.Close)
	return store
}
