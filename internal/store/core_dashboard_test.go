package store

import (
	"context"
	"errors"
	"path/filepath"
	"slices"
	"testing"
	"time"
)

func TestCoreDashboardPeriodsSnapshotsAndCashFlow(t *testing.T) {
	ctx := context.Background()
	s, err := OpenCore(ctx, filepath.Join(t.TempDir(), "dashboard.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	var objects []FinancialObject
	for _, input := range []SaveFinancialObject{
		{ObjectType: "asset", Name: "Woning"}, {ObjectType: "debt", Name: "Hypotheek"},
		{ObjectType: "reserve", Name: "Nieuw dak"}, {ObjectType: "expense", Name: "Rente"},
		{ObjectType: "expense", Name: "Boodschappen"}, {ObjectType: "asset", Name: "Zonder waardering"},
	} {
		item, err := s.CreateFinancialObject(ctx, input)
		if err != nil {
			t.Fatal(err)
		}
		objects = append(objects, item)
	}
	for _, value := range []struct {
		object int
		date   string
		amount int64
	}{
		{0, "2025-12-01", 50000000}, {0, "2026-01-01", 51000000},
		{0, "2026-02-01", 52000000}, {0, "2026-02-01", 52500000}, {0, "2027-01-01", 60000000},
		{1, "2026-01-01", 10000000}, {1, "2026-02-01", 9000000},
		{2, "2026-01-01", 250000}, {2, "2026-02-01", 300000},
		{3, "2026-01-01", 10000}, {3, "2026-02-01", 12000},
	} {
		if _, err := s.CreateObjectValue(ctx, objects[value.object].ID, SaveObjectValue{ValueDate: value.date, ValueCents: cents(value.amount)}); err != nil {
			t.Fatal(err)
		}
	}
	for _, input := range []SaveCoreTransaction{
		{TransactionDate: "2026-01-01", AmountCents: cents(300000), Allocations: []TransactionObjectInput{{FinancialObjectID: objects[0].ID}}},
		{TransactionDate: "2026-01-31", AmountCents: cents(-100000), Allocations: []TransactionObjectInput{{FinancialObjectID: objects[1].ID, AmountCents: cents(-70000)}, {FinancialObjectID: objects[3].ID, AmountCents: cents(-30000)}}},
		{TransactionDate: "2026-02-01", AmountCents: cents(400000), Allocations: []TransactionObjectInput{{FinancialObjectID: objects[0].ID, AmountCents: cents(400000)}}},
		{TransactionDate: "2026-02-28", AmountCents: cents(-50000), Allocations: []TransactionObjectInput{{FinancialObjectID: objects[4].ID, AmountCents: cents(-30000)}, {FinancialObjectID: objects[0].ID}}},
		{TransactionDate: "2026-03-01", AmountCents: cents(50000)},
		{TransactionDate: "2027-01-01", AmountCents: cents(-99999)},
	} {
		if _, err := s.CreateCoreTransaction(ctx, input); err != nil {
			t.Fatal(err)
		}
	}
	jan, err := s.Dashboard(ctx, 2026, 1)
	if err != nil {
		t.Fatal(err)
	}
	if jan.From != "2026-01-01" || jan.To != "2026-01-31" || jan.NetWorthCents != 41000000 {
		t.Fatalf("January snapshot: %+v", jan)
	}
	if jan.ObjectGroups[0].Count != 2 || jan.ObjectGroups[0].ValuedCount != 1 || jan.ObjectGroups[0].ValueCents != 51000000 || jan.ObjectGroups[0].PeriodAmountCents != 0 {
		t.Fatalf("missing valuation/context link: %+v", jan.ObjectGroups[0])
	}
	if jan.ObjectGroups[1].PeriodAmountCents != -70000 || jan.ObjectGroups[2].ValueCents != 250000 || jan.ObjectGroups[3].PeriodAmountCents != -30000 {
		t.Fatalf("planning and allocation totals: %+v", jan.ObjectGroups)
	}
	if jan.Transactions.Count != 2 || jan.Transactions.IncomeCount != 1 || jan.Transactions.ExpenseCount != 1 || jan.Transactions.IncomeCents != 300000 || jan.Transactions.ExpenseCents != -100000 || jan.Transactions.BalanceCents != 200000 || jan.Transactions.UnallocatedCount != 1 {
		t.Fatalf("January flows must not duplicate split/context links: %+v", jan.Transactions)
	}
	feb, err := s.Dashboard(ctx, 2026, 2)
	if err != nil || feb.NetWorthCents != 43500000 || feb.ObjectGroups[0].ValueCents != 52500000 || feb.ObjectGroups[2].ValueCents != 300000 || feb.ObjectGroups[0].PeriodAmountCents != 400000 {
		t.Fatalf("February date cutoff/tie-break/planning: %+v, %v", feb, err)
	}
	if feb.Transactions.Count != 2 || feb.Transactions.BalanceCents != 350000 || feb.Transactions.UnallocatedCount != 1 {
		t.Fatalf("February flows: %+v", feb.Transactions)
	}
	whole, err := s.Dashboard(ctx, 2026, 0)
	if err != nil || whole.From != "2026-01-01" || whole.To != "2026-12-31" || whole.Transactions.Count != 5 || whole.Transactions.IncomeCents != 750000 || whole.Transactions.ExpenseCents != -150000 || whole.Transactions.BalanceCents != 600000 {
		t.Fatalf("whole year must include March but exclude 2027: %+v, %v", whole, err)
	}
	if !slices.Contains(whole.AvailableYears, 2025) || !slices.Contains(whole.AvailableYears, 2027) {
		t.Fatalf("years must include valuation-only years: %v", whole.AvailableYears)
	}
	before, err := s.Dashboard(ctx, 2025, 12)
	if err != nil || before.ObjectGroups[0].ValueCents != 50000000 || before.ObjectGroups[1].ValuedCount != 0 || before.Transactions.Count != 0 {
		t.Fatalf("historical snapshot must not use future debt: %+v, %v", before, err)
	}
	leap, err := s.Dashboard(ctx, 2024, 2)
	if err != nil || leap.To != "2024-02-29" {
		t.Fatalf("leap-year range: %+v, %v", leap, err)
	}
}

func TestCoreDashboardEmptyAndInvalidPeriods(t *testing.T) {
	s, err := OpenCore(context.Background(), filepath.Join(t.TempDir(), "empty.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	item, err := s.Dashboard(context.Background(), 0, 0)
	if err != nil || item.Year != time.Now().Year() || len(item.ObjectGroups) != 5 || item.ObjectGroups[4].ObjectType != "financial" || item.NetWorthCents != 0 || item.Transactions.Count != 0 || len(item.AvailableYears) == 0 {
		t.Fatalf("empty dashboard: %+v, %v", item, err)
	}
	for _, period := range [][2]int{{-1, 0}, {10000, 0}, {2026, -1}, {2026, 13}} {
		if _, err := s.Dashboard(context.Background(), period[0], period[1]); !errors.Is(err, ErrCoreInvalid) {
			t.Fatalf("invalid period accepted: %v", err)
		}
	}
}

func TestCoreBudgetGroupsAllocatedTransactionsByObject(t *testing.T) {
	ctx := context.Background()
	s, err := OpenCore(ctx, filepath.Join(t.TempDir(), "budget.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	last, err := s.CreateFinancialObject(ctx, SaveFinancialObject{ObjectType: "expense", Name: "Boodschappen"})
	if err != nil {
		t.Fatal(err)
	}
	reserve, err := s.CreateFinancialObject(ctx, SaveFinancialObject{ObjectType: "reserve", Name: "Vakantie"})
	if err != nil {
		t.Fatal(err)
	}
	asset, err := s.CreateFinancialObject(ctx, SaveFinancialObject{ObjectType: "asset", Name: "Spaarrekening"})
	if err != nil {
		t.Fatal(err)
	}
	debt, err := s.CreateFinancialObject(ctx, SaveFinancialObject{ObjectType: "debt", Name: "Lening"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateObjectValue(ctx, last.ID, SaveObjectValue{ValueDate: "2026-01-01", ValueCents: cents(50000)}); err != nil {
		t.Fatal(err)
	}
	for _, value := range []struct {
		objectID int64
		cents    int64
	}{{asset.ID, 200000}, {debt.ID, 50000}} {
		if _, err := s.CreateObjectValue(ctx, value.objectID, SaveObjectValue{ValueDate: "2026-01-01", ValueCents: cents(value.cents)}); err != nil {
			t.Fatal(err)
		}
	}
	for _, input := range []SaveCoreTransaction{
		{TransactionDate: "2026-01-10", AmountCents: cents(-30000), Description: "Supermarkt", Allocations: []TransactionObjectInput{{FinancialObjectID: last.ID, AmountCents: cents(-30000)}}},
		{TransactionDate: "2026-01-20", AmountCents: cents(-25000), Description: "Groente", Allocations: []TransactionObjectInput{{FinancialObjectID: last.ID, AmountCents: cents(-25000)}, {FinancialObjectID: reserve.ID}}},
		{TransactionDate: "2026-01-25", AmountCents: cents(20000), Description: "Inleg", Allocations: []TransactionObjectInput{{FinancialObjectID: asset.ID, AmountCents: cents(20000)}}},
		{TransactionDate: "2026-01-26", AmountCents: cents(-10000), Description: "Aflossing", Allocations: []TransactionObjectInput{{FinancialObjectID: debt.ID, AmountCents: cents(-10000)}}},
		{TransactionDate: "2026-02-01", AmountCents: cents(-10000), Description: "Later", Allocations: []TransactionObjectInput{{FinancialObjectID: last.ID, AmountCents: cents(-10000)}}},
	} {
		if _, err := s.CreateCoreTransaction(ctx, input); err != nil {
			t.Fatal(err)
		}
	}
	budget, err := s.Budget(ctx, 2026, 1)
	if err != nil {
		t.Fatal(err)
	}
	if budget.From != "2026-01-01" || budget.To != "2026-01-31" || len(budget.Items) != 4 {
		t.Fatalf("budget period/items: %+v", budget)
	}
	if budget.Summary.AssetChangeCents != 20000 || budget.Summary.DebtChangeCents != -10000 || budget.Summary.ExpenseBudgetCents != 50000 || budget.Summary.ExpenseActualCents != 55000 {
		t.Fatalf("budget summary: %+v", budget.Summary)
	}
	items := map[int64]CoreBudgetItem{}
	for _, item := range budget.Items {
		items[item.FinancialObjectID] = item
	}
	if item := items[last.ID]; item.PeriodAmountCents != -55000 || item.MonthlyMaximumCents == nil || *item.MonthlyMaximumCents != 50000 || len(item.Transactions) != 2 {
		t.Fatalf("expense budget: %+v", item)
	}
	if item := items[reserve.ID]; item.PeriodAmountCents != 0 || item.MonthlyMaximumCents != nil || len(item.Transactions) != 1 || item.Transactions[0].AllocationCents != nil {
		t.Fatalf("context transaction must remain visible without affecting totals: %+v", item)
	}
	if _, err := s.Budget(ctx, 2026, 13); !errors.Is(err, ErrCoreInvalid) {
		t.Fatalf("invalid budget period accepted: %v", err)
	}
}
