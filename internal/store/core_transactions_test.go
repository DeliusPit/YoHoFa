package store

import (
	"context"
	"errors"
	"math"
	"path/filepath"
	"testing"
)

func TestCoreTransactionFlowAndTotals(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "core.db")
	s, err := OpenCore(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	var objects []FinancialObject
	for _, input := range []SaveFinancialObject{
		{ObjectType: "debt", Name: "Hypotheek"},
		{ObjectType: "expense", Name: "Rente"},
		{ObjectType: "asset", Name: "Bankrekening"},
	} {
		object, err := s.CreateFinancialObject(ctx, input)
		if err != nil {
			t.Fatal(err)
		}
		objects = append(objects, object)
	}
	input := SaveCoreTransaction{
		TransactionDate: "2026-10-01", AmountCents: cents(-100000), Description: " Hypotheekbetaling ", Counterparty: " Sparda ",
		Allocations: []TransactionObjectInput{
			{FinancialObjectID: objects[0].ID, AmountCents: cents(-70000)},
			{FinancialObjectID: objects[1].ID, AmountCents: cents(-30000)},
			{FinancialObjectID: objects[2].ID},
		},
	}
	expense, err := s.CreateCoreTransaction(ctx, input)
	if err != nil || len(expense.Allocations) != 3 || !expense.FullyAllocated || expense.Description != "Hypotheekbetaling" || expense.Counterparty != "Sparda" {
		t.Fatalf("split transaction: %+v, %v", expense, err)
	}
	income, err := s.CreateCoreTransaction(ctx, SaveCoreTransaction{TransactionDate: "2026-10-02", AmountCents: cents(250000), Description: "Pensioen"})
	if err != nil || income.FullyAllocated || income.UnallocatedAmountCents != 250000 || len(income.Allocations) != 0 {
		t.Fatalf("unclassified income: %+v, %v", income, err)
	}
	page, err := s.CoreTransactions(ctx, CoreTransactionFilter{Limit: 1})
	if err != nil || page.Total != 2 || len(page.Items) != 1 || page.Items[0].ID != income.ID || page.IncomeCents != 250000 || page.ExpenseCents != -100000 || page.BalanceCents != 150000 {
		t.Fatalf("totals must span all pages and not duplicate context/split links: %+v, %v", page, err)
	}
	page, err = s.CoreTransactions(ctx, CoreTransactionFilter{Direction: "expense", Offset: 0})
	if err != nil || page.Total != 1 || len(page.Items[0].Allocations) != 3 || page.IncomeCents != 0 || page.ExpenseCents != -100000 {
		t.Fatalf("expense filter: %+v, %v", page, err)
	}
	page, err = s.CoreTransactions(ctx, CoreTransactionFilter{Limit: 1, Offset: 1})
	if err != nil || len(page.Items) != 1 || page.Items[0].ID != expense.ID {
		t.Fatalf("pagination: %+v, %v", page, err)
	}
	if err := s.DeleteFinancialObject(ctx, objects[2].ID); !errors.Is(err, ErrCoreConflict) {
		t.Fatalf("context link must protect the object: %v", err)
	}
	input.Allocations = []TransactionObjectInput{{FinancialObjectID: objects[0].ID, AmountCents: cents(-40000)}}
	updated, err := s.UpdateCoreTransaction(ctx, expense.ID, input)
	if err != nil || len(updated.Allocations) != 1 || updated.UnallocatedAmountCents != -60000 || updated.FullyAllocated {
		t.Fatalf("partial allocation and replacement: %+v, %v", updated, err)
	}
	s.Close()
	reopened, err := OpenCore(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	stored, err := reopened.CoreTransaction(ctx, expense.ID)
	if err != nil || len(stored.Allocations) != 1 || stored.UnallocatedAmountCents != -60000 {
		t.Fatalf("transaction must survive restart: %+v, %v", stored, err)
	}
	if err := reopened.DeleteCoreTransaction(ctx, expense.ID); err != nil {
		t.Fatal(err)
	}
	stored, err = reopened.CoreTransaction(ctx, expense.ID)
	if err != nil || !stored.Hidden {
		t.Fatalf("transaction must be hidden, not deleted: %+v, %v", stored, err)
	}
	page, err = reopened.CoreTransactions(ctx, CoreTransactionFilter{})
	if err != nil || page.Total != 1 {
		t.Fatalf("visible list must exclude hidden transaction: %+v, %v", page, err)
	}
	page, err = reopened.CoreTransactions(ctx, CoreTransactionFilter{Visibility: "hidden"})
	if err != nil || page.Total != 1 || len(page.Items) != 1 || !page.Items[0].Hidden {
		t.Fatalf("hidden list must include hidden transaction: %+v, %v", page, err)
	}
}

func TestCoreTransactionValidationAndAtomicWrites(t *testing.T) {
	ctx := context.Background()
	s, err := OpenCore(ctx, filepath.Join(t.TempDir(), "core.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	object, err := s.CreateFinancialObject(ctx, SaveFinancialObject{ObjectType: "expense", Name: "Boodschappen"})
	if err != nil {
		t.Fatal(err)
	}
	base := SaveCoreTransaction{TransactionDate: "2026-10-01", AmountCents: cents(-8342), Description: "ALDI", Allocations: []TransactionObjectInput{{FinancialObjectID: object.ID, AmountCents: cents(-8342)}}}
	item, err := s.CreateCoreTransaction(ctx, base)
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name  string
		input SaveCoreTransaction
		want  error
	}{
		{"missing amount", SaveCoreTransaction{TransactionDate: "2026-10-01"}, ErrCoreInvalid},
		{"zero", SaveCoreTransaction{TransactionDate: "2026-10-01", AmountCents: cents(0)}, ErrCoreInvalid},
		{"invalid date", SaveCoreTransaction{TransactionDate: "2026-02-30", AmountCents: cents(1)}, ErrCoreInvalid},
		{"wrong sign", SaveCoreTransaction{TransactionDate: "2026-10-01", AmountCents: cents(-100), Allocations: []TransactionObjectInput{{FinancialObjectID: object.ID, AmountCents: cents(10)}}}, ErrCoreInvalid},
		{"over allocated", SaveCoreTransaction{TransactionDate: "2026-10-01", AmountCents: cents(-100), Allocations: []TransactionObjectInput{{FinancialObjectID: object.ID, AmountCents: cents(-101)}}}, ErrCoreInvalid},
		{"missing object", SaveCoreTransaction{TransactionDate: "2026-10-01", AmountCents: cents(100), Allocations: []TransactionObjectInput{{FinancialObjectID: 999}}}, ErrCoreNotFound},
		{"duplicate object", SaveCoreTransaction{TransactionDate: "2026-10-01", AmountCents: cents(100), Allocations: []TransactionObjectInput{{FinancialObjectID: object.ID}, {FinancialObjectID: object.ID}}}, ErrCoreInvalid},
	} {
		t.Run(test.name, func(t *testing.T) {
			if _, err := s.UpdateCoreTransaction(ctx, item.ID, test.input); !errors.Is(err, test.want) {
				t.Fatalf("invalid replacement: %v, want %v", err, test.want)
			}
			stored, err := s.CoreTransaction(ctx, item.ID)
			if err != nil || stored.AmountCents != -8342 || len(stored.Allocations) != 1 || !stored.FullyAllocated {
				t.Fatalf("failed update must preserve transaction and links: %+v, %v", stored, err)
			}
		})
	}
	// Simulate a failure after metadata update and link deletion, inside the write.
	if _, err := s.db.ExecContext(ctx, `CREATE TRIGGER fail_transaction_link BEFORE INSERT ON transaction_financial_objects BEGIN SELECT RAISE(ABORT, 'test failure'); END`); err != nil {
		t.Fatal(err)
	}
	changed := base
	changed.Description = "Changed"
	if _, err := s.UpdateCoreTransaction(ctx, item.ID, changed); err == nil {
		t.Fatal("trigger failure must reject update")
	}
	if _, err := s.CreateCoreTransaction(ctx, changed); err == nil {
		t.Fatal("trigger failure must reject create")
	}
	stored, err := s.CoreTransaction(ctx, item.ID)
	if err != nil || stored.Description != "ALDI" || len(stored.Allocations) != 1 {
		t.Fatalf("write transaction did not roll back: %+v, %v", stored, err)
	}
	page, err := s.CoreTransactions(ctx, CoreTransactionFilter{})
	if err != nil || page.Total != 1 {
		t.Fatalf("failed create must not leave metadata: %+v, %v", page, err)
	}
}

func TestCoreAllocationBounds(t *testing.T) {
	for _, amount := range []int64{math.MinInt64, math.MaxInt64} {
		input := SaveCoreTransaction{TransactionDate: "2026-10-01", AmountCents: cents(amount), Allocations: []TransactionObjectInput{{FinancialObjectID: 1, AmountCents: cents(amount)}}}
		if _, err := normalizeCoreTransaction(input); err != nil {
			t.Fatalf("full int64 allocation rejected: %v", err)
		}
		input.Allocations = append(input.Allocations, TransactionObjectInput{FinancialObjectID: 2, AmountCents: cents(amount)})
		if _, err := normalizeCoreTransaction(input); !errors.Is(err, ErrCoreInvalid) {
			t.Fatalf("overflow-sized allocation accepted: %v", err)
		}
	}
}
