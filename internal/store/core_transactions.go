package store

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

type TransactionObjectInput struct {
	FinancialObjectID int64  `json:"financialObjectId"`
	AmountCents       *int64 `json:"amountCents"`
}

type TransactionObjectLink struct {
	FinancialObjectID int64  `json:"financialObjectId"`
	Name              string `json:"name"`
	ObjectType        string `json:"objectType"`
	AmountCents       *int64 `json:"amountCents"`
}

type CoreTransaction struct {
	ID                     int64                   `json:"id"`
	TransactionDate        string                  `json:"transactionDate"`
	AmountCents            int64                   `json:"amountCents"`
	Description            string                  `json:"description"`
	Counterparty           string                  `json:"counterparty"`
	Status                 string                  `json:"status"`
	Hidden                 bool                    `json:"hidden"`
	Allocations            []TransactionObjectLink `json:"allocations"`
	UnallocatedAmountCents int64                   `json:"unallocatedAmountCents"`
	FullyAllocated         bool                    `json:"fullyAllocated"`
}

type SaveCoreTransaction struct {
	TransactionDate string                   `json:"transactionDate"`
	AmountCents     *int64                   `json:"amountCents"`
	Description     string                   `json:"description"`
	Counterparty    string                   `json:"counterparty"`
	Allocations     []TransactionObjectInput `json:"allocations"`
	ApplyToMatching bool                     `json:"applyToMatching"`
}

type CoreTransactionFilter struct {
	Direction         string
	Visibility        string
	From              string
	To                string
	Search            string
	Sort              string
	Order             string
	FinancialObjectID int64
	Status            string
	Limit             int
	Offset            int
}

type CoreTransactionPage struct {
	Items        []CoreTransaction `json:"items"`
	Total        int               `json:"total"`
	IncomeCents  int64             `json:"incomeCents"`
	ExpenseCents int64             `json:"expenseCents"`
	BalanceCents int64             `json:"balanceCents"`
}

type BulkCoreTransactionClassification struct {
	TransactionIDs    []int64 `json:"transactionIds"`
	FinancialObjectID int64   `json:"financialObjectId"`
}

func normalizeCoreTransaction(input SaveCoreTransaction) (SaveCoreTransaction, error) {
	date, err := time.Parse("2006-01-02", input.TransactionDate)
	if err != nil || date.Format("2006-01-02") != input.TransactionDate || input.AmountCents == nil || *input.AmountCents == 0 {
		return input, fmt.Errorf("%w: vul een geldige transactiedatum en een bedrag ongelijk aan nul in", ErrCoreInvalid)
	}
	input.Description = strings.TrimSpace(input.Description)
	input.Counterparty = strings.TrimSpace(input.Counterparty)
	remaining := *input.AmountCents
	seen := make(map[int64]bool, len(input.Allocations))
	for _, link := range input.Allocations {
		if link.FinancialObjectID <= 0 || seen[link.FinancialObjectID] {
			return input, fmt.Errorf("%w: kies elk financieel object maximaal één keer", ErrCoreInvalid)
		}
		seen[link.FinancialObjectID] = true
		if link.AmountCents == nil {
			continue // Context links do not allocate money.
		}
		amount := *link.AmountCents
		// Compare signed amounts directly; abs(MinInt64) and summation may overflow.
		if (*input.AmountCents > 0 && (amount < 0 || amount > remaining)) ||
			(*input.AmountCents < 0 && (amount > 0 || amount < remaining)) {
			return input, fmt.Errorf("%w: toewijzingen moeten hetzelfde teken hebben en mogen het transactiebedrag niet overschrijden", ErrCoreInvalid)
		}
		remaining -= amount
	}
	return input, nil
}

func (s *Core) CoreTransactions(ctx context.Context, filter CoreTransactionFilter) (CoreTransactionPage, error) {
	page := CoreTransactionPage{Items: []CoreTransaction{}}
	if filter.Direction != "" && filter.Direction != "income" && filter.Direction != "expense" {
		return page, ErrCoreInvalid
	}
	if filter.Visibility == "" {
		filter.Visibility = "visible"
	}
	if filter.Visibility != "visible" && filter.Visibility != "hidden" && filter.Visibility != "all" {
		return page, ErrCoreInvalid
	}
	if filter.Status != "" && !validTransactionStatus(filter.Status) {
		return page, ErrCoreInvalid
	}
	if filter.From != "" {
		if date, err := time.Parse("2006-01-02", filter.From); err != nil || date.Format("2006-01-02") != filter.From {
			return page, ErrCoreInvalid
		}
	}
	if filter.To != "" {
		if date, err := time.Parse("2006-01-02", filter.To); err != nil || date.Format("2006-01-02") != filter.To {
			return page, ErrCoreInvalid
		}
	}
	filter.Search = strings.TrimSpace(filter.Search)
	if filter.Sort == "" {
		filter.Sort = "date"
	}
	if filter.Order == "" {
		filter.Order = "desc"
	}
	if (filter.Sort != "date" && filter.Sort != "description" && filter.Sort != "amount") || (filter.Order != "asc" && filter.Order != "desc") {
		return page, ErrCoreInvalid
	}
	if filter.Limit == 0 {
		filter.Limit = 25
	}
	if filter.Limit < 1 || filter.Limit > 100 || filter.Offset < 0 {
		return page, ErrCoreInvalid
	}
	if filter.FinancialObjectID < 0 {
		return page, ErrCoreInvalid
	}
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return page, err
	}
	defer tx.Rollback()
	where := ` WHERE (? = '' OR (? = 'income' AND amount_cents > 0) OR (? = 'expense' AND amount_cents < 0))
		AND ((? = 'visible' AND hidden = 0) OR (? = 'hidden' AND hidden = 1) OR ? = 'all')
		AND (? = '' OR status = ?)
		AND (? = '' OR transaction_date >= ?)
		AND (? = '' OR transaction_date <= ?)
		AND (? = '' OR lower(coalesce(description, '')) LIKE '%' || lower(?) || '%' OR lower(coalesce(counterparty, '')) LIKE '%' || lower(?) || '%')
		AND (? = 0 OR EXISTS (SELECT 1 FROM transaction_financial_objects l WHERE l.transaction_id = transactions.id AND l.financial_object_id = ?))`
	args := []any{filter.Direction, filter.Direction, filter.Direction, filter.Visibility, filter.Visibility, filter.Visibility, filter.Status, filter.Status, filter.From, filter.From, filter.To, filter.To, filter.Search, filter.Search, filter.Search, filter.FinancialObjectID, filter.FinancialObjectID}
	err = tx.QueryRowContext(ctx, `SELECT count(*),
		coalesce(sum(CASE WHEN amount_cents > 0 THEN amount_cents ELSE 0 END), 0),
		coalesce(sum(CASE WHEN amount_cents < 0 THEN amount_cents ELSE 0 END), 0),
		coalesce(sum(amount_cents), 0) FROM transactions`+where, args...).Scan(
		&page.Total, &page.IncomeCents, &page.ExpenseCents, &page.BalanceCents)
	if err != nil {
		return page, err
	}
	rows, err := tx.QueryContext(ctx, "SELECT id, transaction_date, amount_cents, coalesce(description, ''), coalesce(counterparty, ''), status, hidden FROM transactions"+where+transactionOrderBy(filter)+" LIMIT ? OFFSET ?", append(args, filter.Limit, filter.Offset)...)
	if err != nil {
		return page, err
	}
	for rows.Next() {
		var item CoreTransaction
		if err := rows.Scan(&item.ID, &item.TransactionDate, &item.AmountCents, &item.Description, &item.Counterparty, &item.Status, &item.Hidden); err != nil {
			rows.Close()
			return page, err
		}
		page.Items = append(page.Items, item)
	}
	rowErr := rows.Err()
	rows.Close()
	if rowErr != nil {
		return page, rowErr
	}
	for i := range page.Items {
		if err := loadCoreAllocations(ctx, tx, &page.Items[i]); err != nil {
			return page, err
		}
	}
	return page, tx.Commit()
}

func transactionOrderBy(filter CoreTransactionFilter) string {
	direction := "DESC"
	if filter.Order == "asc" {
		direction = "ASC"
	}
	switch filter.Sort {
	case "description":
		return " ORDER BY lower(coalesce(description, '')) " + direction + ", lower(coalesce(counterparty, '')) " + direction + ", transaction_date DESC, id DESC"
	case "amount":
		return " ORDER BY amount_cents " + direction + ", transaction_date DESC, id DESC"
	default:
		return " ORDER BY transaction_date " + direction + ", id " + direction
	}
}

func readCoreTransaction(ctx context.Context, tx *sql.Tx, id int64) (CoreTransaction, error) {
	var item CoreTransaction
	err := tx.QueryRowContext(ctx, "SELECT id, transaction_date, amount_cents, coalesce(description, ''), coalesce(counterparty, ''), status, hidden FROM transactions WHERE id = ?", id).Scan(
		&item.ID, &item.TransactionDate, &item.AmountCents, &item.Description, &item.Counterparty, &item.Status, &item.Hidden)
	if err != nil {
		return item, coreStoreError(err)
	}
	err = loadCoreAllocations(ctx, tx, &item)
	return item, err
}

func validTransactionStatus(status string) bool {
	return status == "I" || status == "A" || status == "F" || status == "U"
}

func loadCoreAllocations(ctx context.Context, tx *sql.Tx, item *CoreTransaction) error {
	rows, err := tx.QueryContext(ctx, `SELECT o.id, o.name, o.object_type, l.amount_cents
		FROM transaction_financial_objects l JOIN financial_objects o ON o.id = l.financial_object_id
		WHERE l.transaction_id = ? ORDER BY o.name, o.id`, item.ID)
	if err != nil {
		return err
	}
	defer rows.Close()
	item.Allocations = []TransactionObjectLink{}
	item.UnallocatedAmountCents = item.AmountCents
	for rows.Next() {
		var link TransactionObjectLink
		var amount sql.NullInt64
		if err := rows.Scan(&link.FinancialObjectID, &link.Name, &link.ObjectType, &amount); err != nil {
			return err
		}
		if amount.Valid {
			value := amount.Int64
			link.AmountCents = &value
			item.UnallocatedAmountCents -= value
		}
		item.Allocations = append(item.Allocations, link)
	}
	item.FullyAllocated = item.UnallocatedAmountCents == 0
	return rows.Err()
}

func (s *Core) CoreTransaction(ctx context.Context, id int64) (CoreTransaction, error) {
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return CoreTransaction{}, err
	}
	defer tx.Rollback()
	item, err := readCoreTransaction(ctx, tx, id)
	if err != nil {
		return item, err
	}
	return item, tx.Commit()
}

func (s *Core) CreateCoreTransaction(ctx context.Context, input SaveCoreTransaction) (CoreTransaction, error) {
	return s.saveCoreTransaction(ctx, 0, input)
}

func (s *Core) UpdateCoreTransaction(ctx context.Context, id int64, input SaveCoreTransaction) (CoreTransaction, error) {
	if id <= 0 {
		return CoreTransaction{}, ErrCoreInvalid
	}
	return s.saveCoreTransaction(ctx, id, input)
}

func (s *Core) saveCoreTransaction(ctx context.Context, id int64, input SaveCoreTransaction) (CoreTransaction, error) {
	input, err := normalizeCoreTransaction(input)
	if err != nil {
		return CoreTransaction{}, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return CoreTransaction{}, err
	}
	defer tx.Rollback()
	for _, link := range input.Allocations {
		var objectID int64
		if err := tx.QueryRowContext(ctx, "SELECT id FROM financial_objects WHERE id = ?", link.FinancialObjectID).Scan(&objectID); err != nil {
			return CoreTransaction{}, coreStoreError(err)
		}
	}
	status := "I"
	if len(input.Allocations) > 0 {
		status = "U"
	}
	if id == 0 {
		result, err := tx.ExecContext(ctx, "INSERT INTO transactions (transaction_date, amount_cents, description, counterparty, status) VALUES (?, ?, ?, ?, ?)", input.TransactionDate, *input.AmountCents, input.Description, input.Counterparty, status)
		if err != nil {
			return CoreTransaction{}, coreStoreError(err)
		}
		id, err = result.LastInsertId()
		if err != nil {
			return CoreTransaction{}, err
		}
	} else {
		result, err := tx.ExecContext(ctx, "UPDATE transactions SET transaction_date = ?, amount_cents = ?, description = ?, counterparty = ?, status = ? WHERE id = ?", input.TransactionDate, *input.AmountCents, input.Description, input.Counterparty, status, id)
		if err := coreMutationError(result, err); err != nil {
			return CoreTransaction{}, err
		}
		if _, err := tx.ExecContext(ctx, "DELETE FROM transaction_financial_objects WHERE transaction_id = ?", id); err != nil {
			return CoreTransaction{}, err
		}
	}
	for _, link := range input.Allocations {
		if _, err := tx.ExecContext(ctx, "INSERT INTO transaction_financial_objects (transaction_id, financial_object_id, amount_cents) VALUES (?, ?, ?)", id, link.FinancialObjectID, link.AmountCents); err != nil {
			return CoreTransaction{}, coreStoreError(err)
		}
	}
	if input.ApplyToMatching && input.Counterparty != "" && len(input.Allocations) > 0 {
		if err := applyAllocationsToMatchingCounterparty(ctx, tx, id, input); err != nil {
			return CoreTransaction{}, err
		}
	}
	item, err := readCoreTransaction(ctx, tx, id)
	if err != nil {
		return item, err
	}
	return item, tx.Commit()
}

func (s *Core) DeleteCoreTransaction(ctx context.Context, id int64) error {
	return s.SetCoreTransactionHidden(ctx, id, true)
}

func (s *Core) SetCoreTransactionHidden(ctx context.Context, id int64, hidden bool) error {
	result, err := s.db.ExecContext(ctx, "UPDATE transactions SET hidden = ? WHERE id = ?", hidden, id)
	return coreMutationError(result, err)
}

func (s *Core) ApplyAITransactionClassifications(ctx context.Context, allocations map[int64]TransactionObjectInput, failed map[int64]bool) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for transactionID, allocation := range allocations {
		var amount int64
		if err := tx.QueryRowContext(ctx, "SELECT amount_cents FROM transactions WHERE id = ? AND status IN ('I', 'F') AND hidden = 0", transactionID).Scan(&amount); err != nil {
			return coreStoreError(err)
		}
		if allocation.AmountCents == nil {
			allocation.AmountCents = &amount
		}
		var objectID int64
		if err := tx.QueryRowContext(ctx, "SELECT id FROM financial_objects WHERE id = ?", allocation.FinancialObjectID).Scan(&objectID); err != nil {
			return coreStoreError(err)
		}
		if _, err := tx.ExecContext(ctx, "DELETE FROM transaction_financial_objects WHERE transaction_id = ?", transactionID); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, "INSERT INTO transaction_financial_objects (transaction_id, financial_object_id, amount_cents) VALUES (?, ?, ?)", transactionID, allocation.FinancialObjectID, allocation.AmountCents); err != nil {
			return coreStoreError(err)
		}
		if _, err := tx.ExecContext(ctx, "UPDATE transactions SET status = 'A' WHERE id = ?", transactionID); err != nil {
			return coreStoreError(err)
		}
		delete(failed, transactionID)
	}
	for transactionID := range failed {
		result, err := tx.ExecContext(ctx, "UPDATE transactions SET status = 'F' WHERE id = ? AND status IN ('I', 'F') AND hidden = 0", transactionID)
		if err := coreMutationError(result, err); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Core) BulkClassifyCoreTransactions(ctx context.Context, input BulkCoreTransactionClassification) error {
	if input.FinancialObjectID <= 0 || len(input.TransactionIDs) == 0 || len(input.TransactionIDs) > 100 {
		return ErrCoreInvalid
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var objectID int64
	if err := tx.QueryRowContext(ctx, "SELECT id FROM financial_objects WHERE id = ?", input.FinancialObjectID).Scan(&objectID); err != nil {
		return coreStoreError(err)
	}
	amounts := make([]int64, len(input.TransactionIDs))
	seen := make(map[int64]bool, len(input.TransactionIDs))
	for index, transactionID := range input.TransactionIDs {
		if transactionID <= 0 || seen[transactionID] {
			return ErrCoreInvalid
		}
		seen[transactionID] = true
		if err := tx.QueryRowContext(ctx, "SELECT amount_cents FROM transactions WHERE id = ? AND hidden = 0", transactionID).Scan(&amounts[index]); err != nil {
			return coreStoreError(err)
		}
	}
	for index, transactionID := range input.TransactionIDs {
		if _, err := tx.ExecContext(ctx, "DELETE FROM transaction_financial_objects WHERE transaction_id = ?", transactionID); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, "INSERT INTO transaction_financial_objects (transaction_id, financial_object_id, amount_cents) VALUES (?, ?, ?)", transactionID, input.FinancialObjectID, amounts[index]); err != nil {
			return coreStoreError(err)
		}
		if _, err := tx.ExecContext(ctx, "UPDATE transactions SET status = 'U' WHERE id = ?", transactionID); err != nil {
			return coreStoreError(err)
		}
	}
	return tx.Commit()
}

func applyAllocationsToMatchingCounterparty(ctx context.Context, tx *sql.Tx, sourceID int64, input SaveCoreTransaction) error {
	copyWholeAmount := len(input.Allocations) == 1 && input.Allocations[0].AmountCents != nil && *input.Allocations[0].AmountCents == *input.AmountCents
	rows, err := tx.QueryContext(ctx, `SELECT t.id, t.amount_cents FROM transactions t
		WHERE t.id <> ? AND t.hidden = 0 AND t.counterparty = ?
		AND NOT EXISTS (SELECT 1 FROM transaction_financial_objects l WHERE l.transaction_id = t.id)`, sourceID, input.Counterparty)
	if err != nil {
		return err
	}
	defer rows.Close()
	targets := []struct{ id, amount int64 }{}
	for rows.Next() {
		var target struct{ id, amount int64 }
		if err := rows.Scan(&target.id, &target.amount); err != nil {
			return err
		}
		targets = append(targets, target)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	for _, target := range targets {
		if _, err := tx.ExecContext(ctx, "UPDATE transactions SET status = 'U' WHERE id = ?", target.id); err != nil {
			return coreStoreError(err)
		}
		for _, link := range input.Allocations {
			var amount *int64
			if copyWholeAmount {
				value := target.amount
				amount = &value
			}
			if _, err := tx.ExecContext(ctx, "INSERT INTO transaction_financial_objects (transaction_id, financial_object_id, amount_cents) VALUES (?, ?, ?)", target.id, link.FinancialObjectID, amount); err != nil {
				return coreStoreError(err)
			}
		}
	}
	return nil
}
