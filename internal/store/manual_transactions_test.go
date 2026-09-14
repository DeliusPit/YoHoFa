package store

import (
	"context"
	"testing"
)

func TestSQLiteManualTransactionsUseNormalLedger(t *testing.T) {
	db := openTestSQLite(t)
	ctx := context.Background()
	account, err := db.CreateAccount(ctx, SaveAccount{Name: "Kas", Type: "checking"})
	if err != nil {
		t.Fatal(err)
	}
	incomeCategory, err := db.CreateCategory(ctx, CreateCategory{MainID: 1, Name: "Contante ontvangst", Kind: "income", Purpose: "daily"})
	if err != nil {
		t.Fatal(err)
	}
	expenseCategory, err := db.CreateCategory(ctx, CreateCategory{MainID: 3, Name: "Contante aankoop", Kind: "expense", Purpose: "daily"})
	if err != nil {
		t.Fatal(err)
	}

	for _, input := range []CreateManualTransaction{
		{AccountID: account.ID, CategoryID: incomeCategory.ID, BookedOn: "2026-08-18", Description: "Contante ontvangst", AmountCents: 5000},
		{AccountID: account.ID, CategoryID: expenseCategory.ID, BookedOn: "2026-08-18", Description: "Contante aankoop", AmountCents: -1250},
		{AccountID: account.ID, BookedOn: "2026-08-18", Description: "Contante aankoop", AmountCents: -1250},
	} {
		created, err := db.CreateManualTransaction(ctx, input)
		if err != nil {
			t.Fatal(err)
		}
		if created.ImportFingerprint != "" || created.ExternalReference != "" {
			t.Fatal("manual transaction contains import metadata")
		}
	}
	accounts, err := db.Accounts(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range accounts {
		if item.ID == account.ID && item.BalanceCents != 2500 {
			t.Fatalf("cash balance = %d, want 2500", item.BalanceCents)
		}
	}
	if _, err := db.CreateManualTransaction(ctx, CreateManualTransaction{AccountID: account.ID, CategoryID: incomeCategory.ID, BookedOn: "2026-08-18", Description: "Verkeerde richting", AmountCents: -100}); err == nil {
		t.Fatal("income category with negative amount must be rejected")
	}
	if _, err := db.CreateManualTransaction(ctx, CreateManualTransaction{AccountID: account.ID, BookedOn: "2026-02-30", Description: "Ongeldige datum", AmountCents: -100}); err == nil {
		t.Fatal("invalid date must be rejected")
	}
}
