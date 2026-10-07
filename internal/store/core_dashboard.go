package store

import (
	"context"
	"database/sql"
	"fmt"
	"math"
	"time"
)

type DashboardObjectGroup struct {
	ObjectType        string `json:"objectType"`
	Count             int    `json:"count"`
	ValuedCount       int    `json:"valuedCount"`
	ValueCents        int64  `json:"valueCents"`
	PeriodAmountCents int64  `json:"periodAmountCents"`
}

type DashboardTransactionSummary struct {
	Count            int   `json:"count"`
	IncomeCount      int   `json:"incomeCount"`
	ExpenseCount     int   `json:"expenseCount"`
	IncomeCents      int64 `json:"incomeCents"`
	ExpenseCents     int64 `json:"expenseCents"`
	BalanceCents     int64 `json:"balanceCents"`
	UnallocatedCount int   `json:"unallocatedCount"`
}

type CoreDashboard struct {
	Year           int                         `json:"year"`
	Month          int                         `json:"month"`
	From           string                      `json:"from"`
	To             string                      `json:"to"`
	AvailableYears []int                       `json:"availableYears"`
	ObjectGroups   []DashboardObjectGroup      `json:"objectGroups"`
	NetWorthCents  int64                       `json:"netWorthCents"`
	Transactions   DashboardTransactionSummary `json:"transactions"`
}

func (s *Core) Dashboard(ctx context.Context, year, month int) (CoreDashboard, error) {
	if year == 0 {
		year = time.Now().Year()
	}
	if year < 1 || year > 9999 || month < 0 || month > 12 {
		return CoreDashboard{}, fmt.Errorf("%w: kies een geldig jaar en een maand van 1 tot 12, of 0 voor het hele jaar", ErrCoreInvalid)
	}
	from := time.Date(year, time.January, 1, 0, 0, 0, 0, time.UTC)
	to := from.AddDate(1, 0, -1)
	if month > 0 {
		from = time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.UTC)
		to = from.AddDate(0, 1, -1)
	}
	item := CoreDashboard{
		Year: year, Month: month, From: from.Format("2006-01-02"), To: to.Format("2006-01-02"),
		AvailableYears: []int{},
		ObjectGroups:   []DashboardObjectGroup{{ObjectType: "asset"}, {ObjectType: "debt"}, {ObjectType: "reserve"}, {ObjectType: "expense"}},
	}
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return item, err
	}
	defer tx.Rollback()
	rows, err := tx.QueryContext(ctx, `SELECT year FROM (
		SELECT CAST(substr(transaction_date, 1, 4) AS INTEGER) AS year FROM transactions WHERE hidden = 0
		UNION SELECT CAST(substr(value_date, 1, 4) AS INTEGER) FROM object_values
		UNION SELECT ?
	) WHERE year BETWEEN 1 AND 9999 ORDER BY year DESC`, time.Now().Year())
	if err != nil {
		return item, err
	}
	for rows.Next() {
		var value int
		if err := rows.Scan(&value); err != nil {
			rows.Close()
			return item, err
		}
		item.AvailableYears = append(item.AvailableYears, value)
	}
	if err := closeDashboardRows(rows); err != nil {
		return item, err
	}
	rows, err = tx.QueryContext(ctx, `SELECT o.object_type, count(*), count(v.id), coalesce(sum(v.value_cents), 0)
		FROM financial_objects o LEFT JOIN object_values v ON v.id = (
			SELECT id FROM object_values WHERE financial_object_id = o.id AND value_date <= ?
			ORDER BY value_date DESC, id DESC LIMIT 1
		) GROUP BY o.object_type`, item.To)
	if err != nil {
		return item, err
	}
	for rows.Next() {
		var group DashboardObjectGroup
		if err := rows.Scan(&group.ObjectType, &group.Count, &group.ValuedCount, &group.ValueCents); err != nil {
			rows.Close()
			return item, err
		}
		for i := range item.ObjectGroups {
			if item.ObjectGroups[i].ObjectType == group.ObjectType {
				item.ObjectGroups[i] = group
			}
		}
	}
	if err := closeDashboardRows(rows); err != nil {
		return item, err
	}
	assets, debts := item.ObjectGroups[0].ValueCents, item.ObjectGroups[1].ValueCents
	if (debts > 0 && assets < math.MinInt64+debts) || (debts < 0 && assets > math.MaxInt64+debts) {
		return item, fmt.Errorf("Het netto balansbedrag is te groot")
	}
	item.NetWorthCents = assets - debts
	// Explicit allocations are aggregated separately from value snapshots and
	// cash-flow totals; context links must not multiply transaction amounts.
	rows, err = tx.QueryContext(ctx, `SELECT o.object_type, coalesce(sum(l.amount_cents), 0)
		FROM transactions t JOIN transaction_financial_objects l ON l.transaction_id = t.id
		JOIN financial_objects o ON o.id = l.financial_object_id
		WHERE t.hidden = 0 AND t.transaction_date BETWEEN ? AND ? GROUP BY o.object_type`, item.From, item.To)
	if err != nil {
		return item, err
	}
	for rows.Next() {
		var kind string
		var amount int64
		if err := rows.Scan(&kind, &amount); err != nil {
			rows.Close()
			return item, err
		}
		for i := range item.ObjectGroups {
			if item.ObjectGroups[i].ObjectType == kind {
				item.ObjectGroups[i].PeriodAmountCents = amount
			}
		}
	}
	if err := closeDashboardRows(rows); err != nil {
		return item, err
	}
	err = tx.QueryRowContext(ctx, `SELECT count(*),
		coalesce(sum(CASE WHEN amount_cents > 0 THEN 1 ELSE 0 END), 0),
		coalesce(sum(CASE WHEN amount_cents < 0 THEN 1 ELSE 0 END), 0),
		coalesce(sum(CASE WHEN amount_cents > 0 THEN amount_cents ELSE 0 END), 0),
		coalesce(sum(CASE WHEN amount_cents < 0 THEN amount_cents ELSE 0 END), 0),
		coalesce(sum(amount_cents), 0),
		coalesce(sum(CASE WHEN amount_cents <> coalesce((
			SELECT sum(l.amount_cents) FROM transaction_financial_objects l WHERE l.transaction_id = t.id
		), 0) THEN 1 ELSE 0 END), 0)
		FROM transactions t WHERE hidden = 0 AND transaction_date BETWEEN ? AND ?`, item.From, item.To).Scan(
		&item.Transactions.Count, &item.Transactions.IncomeCount, &item.Transactions.ExpenseCount,
		&item.Transactions.IncomeCents, &item.Transactions.ExpenseCents, &item.Transactions.BalanceCents,
		&item.Transactions.UnallocatedCount)
	if err != nil {
		return item, err
	}
	return item, tx.Commit()
}

func closeDashboardRows(rows *sql.Rows) error {
	err := rows.Err()
	closeErr := rows.Close()
	if err != nil {
		return err
	}
	return closeErr
}
