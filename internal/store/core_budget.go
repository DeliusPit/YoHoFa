package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

type CoreBudgetTransaction struct {
	ID              int64  `json:"id"`
	TransactionDate string `json:"transactionDate"`
	AmountCents     int64  `json:"amountCents"`
	Description     string `json:"description"`
	Counterparty    string `json:"counterparty"`
	AllocationCents *int64 `json:"allocationCents"`
}

type CoreBudgetItem struct {
	FinancialObjectID   int64                   `json:"financialObjectId"`
	Name                string                  `json:"name"`
	ObjectType          string                  `json:"objectType"`
	MonthlyMaximumCents *int64                  `json:"monthlyMaximumCents"`
	PeriodAmountCents   int64                   `json:"periodAmountCents"`
	Transactions        []CoreBudgetTransaction `json:"transactions"`
}

type CoreBudgetSummary struct {
	AssetChangeCents   int64 `json:"assetChangeCents"`
	DebtChangeCents    int64 `json:"debtChangeCents"`
	ExpenseBudgetCents int64 `json:"expenseBudgetCents"`
	ExpenseActualCents int64 `json:"expenseActualCents"`
}

type CoreBudget struct {
	Year           int               `json:"year"`
	Month          int               `json:"month"`
	From           string            `json:"from"`
	To             string            `json:"to"`
	AvailableYears []int             `json:"availableYears"`
	Summary        CoreBudgetSummary `json:"summary"`
	Items          []CoreBudgetItem  `json:"items"`
}

func budgetPeriod(year, month int) (time.Time, time.Time, error) {
	if year == 0 {
		year = time.Now().Year()
	}
	if year < 1 || year > 9999 || month < 0 || month > 12 {
		return time.Time{}, time.Time{}, fmt.Errorf("%w: kies een geldig jaar en een maand van 1 tot 12, of 0 voor het hele jaar", ErrCoreInvalid)
	}
	from := time.Date(year, time.January, 1, 0, 0, 0, 0, time.UTC)
	to := from.AddDate(1, 0, -1)
	if month > 0 {
		from = time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.UTC)
		to = from.AddDate(0, 1, -1)
	}
	return from, to, nil
}

func (s *Core) Budget(ctx context.Context, year, month int) (CoreBudget, error) {
	from, to, err := budgetPeriod(year, month)
	if err != nil {
		return CoreBudget{}, err
	}
	item := CoreBudget{
		Year: year, Month: month, From: from.Format("2006-01-02"), To: to.Format("2006-01-02"),
		AvailableYears: []int{}, Items: []CoreBudgetItem{},
	}
	if item.Year == 0 {
		item.Year = from.Year()
	}
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return item, err
	}
	defer tx.Rollback()
	rows, err := tx.QueryContext(ctx, `SELECT year FROM (
		SELECT CAST(substr(transaction_date, 1, 4) AS INTEGER) AS year FROM transactions WHERE hidden = 0
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
	rows, err = tx.QueryContext(ctx, `SELECT v.value_cents
		FROM financial_objects o JOIN object_values v ON v.id = (
			SELECT id FROM object_values WHERE financial_object_id = o.id AND value_date <= ?
			ORDER BY value_date DESC, id DESC LIMIT 1
		) WHERE o.object_type = 'expense'`, item.To)
	if err != nil {
		return item, err
	}
	for rows.Next() {
		var value int64
		if err := rows.Scan(&value); err != nil {
			rows.Close()
			return item, err
		}
		if value < 0 {
			value = -value
		}
		item.Summary.ExpenseBudgetCents += value
	}
	if err := closeDashboardRows(rows); err != nil {
		return item, err
	}
	if item.Month == 0 {
		item.Summary.ExpenseBudgetCents *= 12
	}
	rows, err = tx.QueryContext(ctx, `SELECT o.id, o.name, o.object_type, v.value_cents, coalesce(sum(l.amount_cents), 0)
		FROM financial_objects o
		JOIN transaction_financial_objects l ON l.financial_object_id = o.id
		JOIN transactions t ON t.id = l.transaction_id
		LEFT JOIN object_values v ON v.id = (
			SELECT id FROM object_values WHERE financial_object_id = o.id AND value_date <= ?
			ORDER BY value_date DESC, id DESC LIMIT 1
		)
		WHERE t.hidden = 0 AND t.transaction_date BETWEEN ? AND ?
		GROUP BY o.id, o.name, o.object_type, v.value_cents ORDER BY o.object_type, lower(o.name), o.id`, item.To, item.From, item.To)
	if err != nil {
		return item, err
	}
	byID := make(map[int64]int)
	for rows.Next() {
		var entry CoreBudgetItem
		var maximum sql.NullInt64
		if err := rows.Scan(&entry.FinancialObjectID, &entry.Name, &entry.ObjectType, &maximum, &entry.PeriodAmountCents); err != nil {
			rows.Close()
			return item, err
		}
		if entry.ObjectType == "expense" && maximum.Valid {
			value := maximum.Int64
			if value < 0 {
				value = -value
			}
			entry.MonthlyMaximumCents = &value
		}
		entry.Transactions = []CoreBudgetTransaction{}
		switch entry.ObjectType {
		case "asset":
			item.Summary.AssetChangeCents += entry.PeriodAmountCents
		case "debt":
			item.Summary.DebtChangeCents += entry.PeriodAmountCents
		case "expense":
			item.Summary.ExpenseActualCents -= entry.PeriodAmountCents
		}
		byID[entry.FinancialObjectID] = len(item.Items)
		item.Items = append(item.Items, entry)
	}
	if err := closeDashboardRows(rows); err != nil {
		return item, err
	}
	rows, err = tx.QueryContext(ctx, `SELECT l.financial_object_id, t.id, t.transaction_date, t.amount_cents,
		coalesce(t.description, ''), coalesce(t.counterparty, ''), l.amount_cents
		FROM transaction_financial_objects l JOIN transactions t ON t.id = l.transaction_id
		WHERE t.hidden = 0 AND t.transaction_date BETWEEN ? AND ?
		ORDER BY l.financial_object_id, t.transaction_date DESC, t.id DESC`, item.From, item.To)
	if err != nil {
		return item, err
	}
	for rows.Next() {
		var objectID int64
		var entry CoreBudgetTransaction
		var allocation sql.NullInt64
		if err := rows.Scan(&objectID, &entry.ID, &entry.TransactionDate, &entry.AmountCents, &entry.Description, &entry.Counterparty, &allocation); err != nil {
			rows.Close()
			return item, err
		}
		if allocation.Valid {
			value := allocation.Int64
			entry.AllocationCents = &value
		}
		if index, ok := byID[objectID]; ok {
			item.Items[index].Transactions = append(item.Items[index].Transactions, entry)
		}
	}
	if err := closeDashboardRows(rows); err != nil {
		return item, err
	}
	return item, tx.Commit()
}
