package store

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"strings"

	_ "modernc.org/sqlite"
)

type SQLite struct {
	db *sql.DB
}

func OpenSQLite(ctx context.Context, databasePath string) (*SQLite, error) {
	db, err := sql.Open("sqlite", databasePath)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)

	store := &SQLite{db: db}
	if err := store.Ping(ctx); err != nil {
		db.Close()
		return nil, err
	}
	if err := store.ensureSchema(ctx); err != nil {
		db.Close()
		return nil, err
	}
	if err := store.seedDefaults(ctx); err != nil {
		db.Close()
		return nil, err
	}
	return store, nil
}

func (s *SQLite) Close() {
	s.db.Close()
}

func (s *SQLite) Ping(ctx context.Context) error {
	return s.db.PingContext(ctx)
}

func (s *SQLite) Summary(ctx context.Context) (Summary, error) {
	var summary Summary
	err := s.db.QueryRowContext(ctx, `
		select
			(select count(*) from households),
			(select count(*) from accounts),
			(select count(*) from transactions where hidden = false)
	`).Scan(&summary.Households, &summary.Accounts, &summary.Transactions)
	return summary, err
}

func (s *SQLite) MainCategories(ctx context.Context) ([]MainCategory, error) {
	rows, err := s.db.QueryContext(ctx, `
		select id, name, sort_order
		from main_categories
		order by sort_order, name
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := []MainCategory{}
	for rows.Next() {
		var item MainCategory
		if err := rows.Scan(&item.ID, &item.Name, &item.SortOrder); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *SQLite) CreateMainCategory(ctx context.Context, input CreateMainCategory) (MainCategory, error) {
	name := trimRequired(input.Name, "name")
	if name.err != nil {
		return MainCategory{}, name.err
	}

	sortOrder, err := s.nextSortOrder(ctx, "main_categories", "")
	if err != nil {
		return MainCategory{}, err
	}
	result, err := s.db.ExecContext(ctx, `insert into main_categories (name, sort_order) values (?, ?)`, name.value, sortOrder)
	if err != nil {
		return MainCategory{}, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return MainCategory{}, err
	}
	return MainCategory{ID: id, Name: name.value, SortOrder: sortOrder}, nil
}

func (s *SQLite) UpdateMainCategory(ctx context.Context, id int64, input UpdateMainCategory) (MainCategory, error) {
	name := trimRequired(input.Name, "name")
	if name.err != nil {
		return MainCategory{}, name.err
	}
	result, err := s.db.ExecContext(ctx, `update main_categories set name = ? where id = ?`, name.value, id)
	if err != nil {
		return MainCategory{}, err
	}
	if affected, _ := result.RowsAffected(); affected == 0 {
		return MainCategory{}, fmt.Errorf("main category not found")
	}
	return s.mainCategory(ctx, id)
}

func (s *SQLite) DeleteMainCategory(ctx context.Context, id int64) error {
	var categoryCount int
	if err := s.db.QueryRowContext(ctx, `select count(*) from categories where main_category_id = ?`, id).Scan(&categoryCount); err != nil {
		return err
	}
	if categoryCount > 0 {
		return fmt.Errorf("main category still has %d subcategories", categoryCount)
	}
	result, err := s.db.ExecContext(ctx, `delete from main_categories where id = ?`, id)
	if err != nil {
		return err
	}
	if affected, _ := result.RowsAffected(); affected == 0 {
		return fmt.Errorf("main category not found")
	}
	return nil
}

func (s *SQLite) Categories(ctx context.Context) ([]Category, error) {
	rows, err := s.db.QueryContext(ctx, `
		select id, coalesce(main_category_id, 0), name, kind, purpose, sort_order
		from categories
		order by sort_order, name
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := []Category{}
	for rows.Next() {
		var item Category
		if err := rows.Scan(&item.ID, &item.MainID, &item.Name, &item.Kind, &item.Purpose, &item.SortOrder); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *SQLite) CreateCategory(ctx context.Context, input CreateCategory) (Category, error) {
	name := trimRequired(input.Name, "name")
	if name.err != nil {
		return Category{}, name.err
	}
	kind, purpose, err := normalizeCategoryInput(input.Kind, input.Purpose)
	if err != nil {
		return Category{}, err
	}
	sortOrder, err := s.nextSortOrder(ctx, "categories", "where main_category_id = ?", input.MainID)
	if err != nil {
		return Category{}, err
	}
	result, err := s.db.ExecContext(ctx, `
		insert into categories (main_category_id, name, kind, purpose, sort_order)
		values (?, ?, ?, ?, ?)
	`, input.MainID, name.value, kind, purpose, sortOrder)
	if err != nil {
		return Category{}, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return Category{}, err
	}
	return Category{ID: id, MainID: input.MainID, Name: name.value, Kind: kind, Purpose: purpose, SortOrder: sortOrder}, nil
}

func (s *SQLite) UpdateCategory(ctx context.Context, id int64, input UpdateCategory) (Category, error) {
	name := trimRequired(input.Name, "name")
	if name.err != nil {
		return Category{}, name.err
	}
	kind, purpose, err := normalizeCategoryInput(input.Kind, input.Purpose)
	if err != nil {
		return Category{}, err
	}
	result, err := s.db.ExecContext(ctx, `
		update categories
		set main_category_id = ?, name = ?, kind = ?, purpose = ?
		where id = ?
	`, input.MainID, name.value, kind, purpose, id)
	if err != nil {
		return Category{}, err
	}
	if affected, _ := result.RowsAffected(); affected == 0 {
		return Category{}, fmt.Errorf("category not found")
	}
	return s.category(ctx, id)
}

func (s *SQLite) DeleteCategory(ctx context.Context, id int64) error {
	var transactionCount int
	if err := s.db.QueryRowContext(ctx, `select count(*) from transactions where category_id = ?`, id).Scan(&transactionCount); err != nil {
		return err
	}
	if transactionCount > 0 {
		return fmt.Errorf("category is used by %d transactions", transactionCount)
	}
	result, err := s.db.ExecContext(ctx, `delete from categories where id = ?`, id)
	if err != nil {
		return err
	}
	if affected, _ := result.RowsAffected(); affected == 0 {
		return fmt.Errorf("category not found")
	}
	return nil
}

func (s *SQLite) Accounts(ctx context.Context) ([]Account, error) {
	rows, err := s.db.QueryContext(ctx, `
		select accounts.id, accounts.name, coalesce(accounts.iban, ''), accounts.account_type, coalesce(accounts.owner_name, ''), accounts.bank_profile, accounts.sort_order,
			accounts.opening_balance_cents + coalesce(sum(case when transactions.hidden = false then transactions.amount_cents else 0 end), 0) as balance_cents
		from accounts
		left join transactions on transactions.account_id = accounts.id
		group by accounts.id
		order by accounts.sort_order, accounts.name
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := []Account{}
	for rows.Next() {
		var item Account
		if err := rows.Scan(&item.ID, &item.Name, &item.IBAN, &item.Type, &item.Owner, &item.BankProfile, &item.SortOrder, &item.BalanceCents); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *SQLite) CreateAccount(ctx context.Context, input SaveAccount) (Account, error) {
	account, err := normalizeAccountInput(input)
	if err != nil {
		return Account{}, err
	}
	account.SortOrder, err = s.nextSortOrder(ctx, "accounts", "")
	if err != nil {
		return Account{}, err
	}
	result, err := s.db.ExecContext(ctx, `
		insert into accounts (name, iban, account_type, owner_name, bank_profile, sort_order)
		values (?, ?, ?, ?, ?, ?)
	`, account.Name, account.IBAN, account.Type, account.Owner, account.BankProfile, account.SortOrder)
	if err != nil {
		return Account{}, err
	}
	account.ID, err = result.LastInsertId()
	return account, err
}

func (s *SQLite) UpdateAccount(ctx context.Context, id int64, input SaveAccount) (Account, error) {
	account, err := normalizeAccountInput(input)
	if err != nil {
		return Account{}, err
	}
	result, err := s.db.ExecContext(ctx, `
		update accounts
		set name = ?, iban = ?, account_type = ?, owner_name = ?, bank_profile = ?
		where id = ?
	`, account.Name, account.IBAN, account.Type, account.Owner, account.BankProfile, id)
	if err != nil {
		return Account{}, err
	}
	if affected, _ := result.RowsAffected(); affected == 0 {
		return Account{}, fmt.Errorf("account not found")
	}
	return s.account(ctx, id)
}

func (s *SQLite) DeleteAccount(ctx context.Context, id int64) error {
	var transactionCount int
	if err := s.db.QueryRowContext(ctx, `select count(*) from transactions where account_id = ?`, id).Scan(&transactionCount); err != nil {
		return err
	}
	if transactionCount > 0 {
		return fmt.Errorf("account has %d transactions", transactionCount)
	}
	result, err := s.db.ExecContext(ctx, `delete from accounts where id = ?`, id)
	if err != nil {
		return err
	}
	if affected, _ := result.RowsAffected(); affected == 0 {
		return fmt.Errorf("account not found")
	}
	return nil
}

func (s *SQLite) ImportTransactions(ctx context.Context, accountID int64, transactions []ImportedTransaction) (ImportResult, error) {
	result := ImportResult{Total: len(transactions)}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return ImportResult{}, err
	}
	defer tx.Rollback()

	occurrences := make(map[string]int)
	for _, transaction := range transactions {
		fingerprint := strings.TrimSpace(transaction.ImportFingerprint)
		if fingerprint == "" {
			baseFingerprint := transactionFingerprint(accountID, transaction)
			occurrences[baseFingerprint]++
			fingerprint = transactionFingerprintOccurrence(baseFingerprint, occurrences[baseFingerprint])
		}

		execResult, err := tx.ExecContext(ctx, `
			insert or ignore into transactions (
				account_id, booked_on, description, counterparty, counterparty_iban,
				amount_cents, external_reference, import_fingerprint
			) values (?, ?, ?, ?, ?, ?, ?, ?)
		`, accountID, transaction.BookedOn, transaction.Description, transaction.Counterparty, transaction.CounterpartyIBAN, transaction.AmountCents, transaction.ExternalReference, fingerprint)
		if err != nil {
			return ImportResult{}, err
		}
		if affected, _ := execResult.RowsAffected(); affected == 0 {
			result.Duplicates++
			continue
		}
		result.Imported++
	}

	return result, tx.Commit()
}

func (s *SQLite) Transactions(ctx context.Context, filter TransactionFilter) ([]Transaction, error) {
	if filter.Limit <= 0 {
		filter.Limit = 25
	}
	if filter.Limit > 200 {
		filter.Limit = 200
	}
	if filter.Offset < 0 {
		filter.Offset = 0
	}

	query := `
		select transactions.id, transactions.account_id, coalesce(transactions.category_id, 0), transactions.booked_on, transactions.description,
			coalesce(transactions.counterparty, ''), coalesce(transactions.counterparty_iban, ''), transactions.amount_cents,
			coalesce(transactions.external_reference, ''), coalesce(transactions.import_fingerprint, ''), transactions.hidden
		from transactions
		left join categories on categories.id = transactions.category_id
		where (? = 0 or account_id = ?)
		and (? = 0 or transactions.category_id = ?)
		and (? = 0 or categories.main_category_id = ?)
		and (? = '' or booked_on >= ?)
		and (? = '' or booked_on <= ?)
		and (
			? = ''
			or lower(transactions.description) like '%' || lower(?) || '%'
			or lower(coalesce(transactions.counterparty, '')) like '%' || lower(?) || '%'
			or lower(coalesce(transactions.counterparty_iban, '')) like '%' || lower(?) || '%'
			or lower(coalesce(transactions.external_reference, '')) like '%' || lower(?) || '%'
		)
		and (? = false or transactions.category_id is null)
		and ((? = true and transactions.hidden = true) or (? = false and transactions.hidden = false))
		order by transactions.booked_on desc, transactions.id desc
		limit ? offset ?
	`
	search := strings.TrimSpace(filter.Search)
	rows, err := s.db.QueryContext(ctx, query, filter.AccountID, filter.AccountID, filter.CategoryID, filter.CategoryID, filter.MainCategoryID, filter.MainCategoryID, filter.From, filter.From, filter.To, filter.To, search, search, search, search, search, filter.Uncategorized, filter.HiddenOnly, filter.HiddenOnly, filter.Limit, filter.Offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	transactions := []Transaction{}
	for rows.Next() {
		var transaction Transaction
		if err := rows.Scan(
			&transaction.ID,
			&transaction.AccountID,
			&transaction.CategoryID,
			&transaction.BookedOn,
			&transaction.Description,
			&transaction.Counterparty,
			&transaction.CounterpartyIBAN,
			&transaction.AmountCents,
			&transaction.ExternalReference,
			&transaction.ImportFingerprint,
			&transaction.Hidden,
		); err != nil {
			return nil, err
		}
		transactions = append(transactions, transaction)
	}
	return transactions, rows.Err()
}

func (s *SQLite) TransactionYears(ctx context.Context) ([]int, error) {
	rows, err := s.db.QueryContext(ctx, `
		select distinct cast(substr(booked_on, 1, 4) as integer) as year
		from transactions
		where hidden = false
		order by year desc
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	years := []int{}
	for rows.Next() {
		var year int
		if err := rows.Scan(&year); err != nil {
			return nil, err
		}
		years = append(years, year)
	}
	return years, rows.Err()
}

func (s *SQLite) TransactionSummary(ctx context.Context, filter TransactionFilter) (TransactionSummary, error) {
	query := `
		select count(*), coalesce(sum(transactions.amount_cents), 0)
		from transactions
		left join categories on categories.id = transactions.category_id
		where (? = 0 or account_id = ?)
		and (? = 0 or transactions.category_id = ?)
		and (? = 0 or categories.main_category_id = ?)
		and (? = '' or booked_on >= ?)
		and (? = '' or booked_on <= ?)
		and (
			? = ''
			or lower(transactions.description) like '%' || lower(?) || '%'
			or lower(coalesce(transactions.counterparty, '')) like '%' || lower(?) || '%'
			or lower(coalesce(transactions.counterparty_iban, '')) like '%' || lower(?) || '%'
			or lower(coalesce(transactions.external_reference, '')) like '%' || lower(?) || '%'
		)
		and (? = false or transactions.category_id is null)
		and ((? = true and transactions.hidden = true) or (? = false and transactions.hidden = false))
	`
	search := strings.TrimSpace(filter.Search)
	var summary TransactionSummary
	err := s.db.QueryRowContext(ctx, query, filter.AccountID, filter.AccountID, filter.CategoryID, filter.CategoryID, filter.MainCategoryID, filter.MainCategoryID, filter.From, filter.From, filter.To, filter.To, search, search, search, search, search, filter.Uncategorized, filter.HiddenOnly, filter.HiddenOnly).Scan(&summary.Count, &summary.AmountCents)
	return summary, err
}

func (s *SQLite) SetTransactionHidden(ctx context.Context, id int64, hidden bool) error {
	result, err := s.db.ExecContext(ctx, `update transactions set hidden = ? where id = ?`, hidden, id)
	if err != nil {
		return err
	}
	if affected, _ := result.RowsAffected(); affected == 0 {
		return fmt.Errorf("transaction not found")
	}
	return nil
}

func (s *SQLite) CategorizeTransaction(ctx context.Context, id int64, input CategorizeTransaction) (CategorizeResult, error) {
	if input.CategoryID == 0 {
		return CategorizeResult{}, fmt.Errorf("category is required")
	}

	if !input.ApplyToSameCounterparty {
		result, err := s.db.ExecContext(ctx, `update transactions set category_id = ? where id = ?`, input.CategoryID, id)
		if err != nil {
			return CategorizeResult{}, err
		}
		updated, _ := result.RowsAffected()
		if updated == 0 {
			return CategorizeResult{}, fmt.Errorf("transaction not found")
		}
		return CategorizeResult{Updated: int(updated)}, nil
	}

	var accountID int64
	var counterparty string
	if err := s.db.QueryRowContext(ctx, `select account_id, coalesce(counterparty, '') from transactions where id = ?`, id).Scan(&accountID, &counterparty); err != nil {
		return CategorizeResult{}, fmt.Errorf("transaction not found")
	}
	if strings.TrimSpace(counterparty) == "" {
		result, err := s.db.ExecContext(ctx, `update transactions set category_id = ? where id = ?`, input.CategoryID, id)
		if err != nil {
			return CategorizeResult{}, err
		}
		updated, _ := result.RowsAffected()
		return CategorizeResult{Updated: int(updated)}, nil
	}

	result, err := s.db.ExecContext(ctx, `
		update transactions
		set category_id = ?
		where account_id = ? and counterparty = ? and category_id is null
	`, input.CategoryID, accountID, counterparty)
	if err != nil {
		return CategorizeResult{}, err
	}
	updated, _ := result.RowsAffected()
	return CategorizeResult{Updated: int(updated)}, nil
}

func (s *SQLite) CategoryMonthlyTotals(ctx context.Context, filter CategoryReportFilter) ([]MonthlyTotal, error) {
	rows, err := s.db.QueryContext(ctx, `
		select substr(transactions.booked_on, 1, 7) as month, sum(transactions.amount_cents)
		from transactions
		join categories on categories.id = transactions.category_id
		where (? = 0 or transactions.category_id = ?)
		and (? = 0 or categories.main_category_id = ?)
		and transactions.hidden = false
		group by month
		order by month
	`, filter.CategoryID, filter.CategoryID, filter.MainCategoryID, filter.MainCategoryID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	totals := []MonthlyTotal{}
	for rows.Next() {
		var total MonthlyTotal
		if err := rows.Scan(&total.Month, &total.AmountCents); err != nil {
			return nil, err
		}
		totals = append(totals, total)
	}
	return totals, rows.Err()
}

func (s *SQLite) MainCategoryPeriodTotals(ctx context.Context, filter PeriodReportFilter) ([]CategoryTotal, error) {
	query := `
		select main_categories.id, main_categories.name, 0, '',
			sum(case
				when categories.kind = 'expense' and transactions.amount_cents < 0 then abs(transactions.amount_cents)
				else 0
			end) as amount_cents
		from transactions
		join categories on categories.id = transactions.category_id
		join main_categories on main_categories.id = categories.main_category_id
		where transactions.hidden = false
		and categories.kind = 'expense'
		and transactions.amount_cents < 0
		and (? = '' or transactions.booked_on >= ?)
		and (? = '' or transactions.booked_on <= ?)
		group by main_categories.id, main_categories.name, main_categories.sort_order
		order by amount_cents desc, main_categories.sort_order
	`
	args := []any{filter.From, filter.From, filter.To, filter.To}
	if filter.MainCategoryID != 0 {
		query = `
			select main_categories.id, main_categories.name, categories.id, categories.name,
				sum(case
					when categories.kind = 'income' and transactions.amount_cents > 0 then transactions.amount_cents
					when categories.kind = 'expense' and transactions.amount_cents < 0 then abs(transactions.amount_cents)
					else 0
				end) as amount_cents
			from transactions
			join categories on categories.id = transactions.category_id
			join main_categories on main_categories.id = categories.main_category_id
			where transactions.hidden = false
			and main_categories.id = ?
			and (
				(categories.kind = 'income' and transactions.amount_cents > 0)
				or (categories.kind = 'expense' and transactions.amount_cents < 0)
			)
			and (? = '' or transactions.booked_on >= ?)
			and (? = '' or transactions.booked_on <= ?)
			group by main_categories.id, main_categories.name, categories.id, categories.name, categories.sort_order
			order by amount_cents desc, categories.sort_order
		`
		args = []any{filter.MainCategoryID, filter.From, filter.From, filter.To, filter.To}
	}

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	totals := []CategoryTotal{}
	for rows.Next() {
		var total CategoryTotal
		if err := rows.Scan(&total.MainCategoryID, &total.MainCategoryName, &total.CategoryID, &total.CategoryName, &total.AmountCents); err != nil {
			return nil, err
		}
		totals = append(totals, total)
	}
	return totals, rows.Err()
}

func transactionFingerprint(accountID int64, transaction ImportedTransaction) string {
	data := fmt.Sprintf("%d|%s|%d|%s|%s|%s", accountID, transaction.BookedOn, transaction.AmountCents, transaction.Description, transaction.CounterpartyIBAN, transaction.ExternalReference)
	hash := sha256.Sum256([]byte(data))
	return hex.EncodeToString(hash[:])
}

func transactionFingerprintOccurrence(baseFingerprint string, occurrence int) string {
	if occurrence <= 1 {
		return baseFingerprint
	}
	hash := sha256.Sum256([]byte(fmt.Sprintf("%s|%d", baseFingerprint, occurrence)))
	return hex.EncodeToString(hash[:])
}

func (s *SQLite) ensureSchema(ctx context.Context) error {
	if _, err := s.db.ExecContext(ctx, sqliteSchema); err != nil {
		return err
	}
	return s.ensureColumn(ctx, "transactions", "hidden", "integer not null default 0")
}

func (s *SQLite) ensureColumn(ctx context.Context, table string, column string, definition string) error {
	rows, err := s.db.QueryContext(ctx, fmt.Sprintf("pragma table_info(%s)", table))
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var cid int
		var name, dataType string
		var notNull int
		var defaultValue any
		var pk int
		if err := rows.Scan(&cid, &name, &dataType, &notNull, &defaultValue, &pk); err != nil {
			return err
		}
		if name == column {
			return rows.Err()
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, fmt.Sprintf("alter table %s add column %s %s", table, column, definition))
	return err
}

func (s *SQLite) seedDefaults(ctx context.Context) error {
	if _, err := s.db.ExecContext(ctx, `insert or ignore into households (id, name) values (1, 'Huishouden')`); err != nil {
		return err
	}
	mainCategoryCount, err := s.tableCount(ctx, "main_categories")
	if err != nil {
		return err
	}
	if mainCategoryCount == 0 {
		for _, item := range defaultMainCategories() {
			if _, err := s.db.ExecContext(ctx, `insert into main_categories (id, name, sort_order) values (?, ?, ?)`, item.ID, item.Name, item.SortOrder); err != nil {
				return err
			}
		}
	}
	accountCount, err := s.tableCount(ctx, "accounts")
	if err != nil {
		return err
	}
	if accountCount == 0 {
		if _, err := s.db.ExecContext(ctx, `
			insert into accounts (id, household_id, name, iban, account_type, owner_name, bank_profile, sort_order)
			values (1, 1, 'Triodos betaalrekening', 'NL18TRIO0254363105', 'checking', 'Huishouden', 'triodos', 10)
		`); err != nil {
			return err
		}
	}
	categoryCount, err := s.tableCount(ctx, "categories")
	if err != nil {
		return err
	}
	if categoryCount == 0 {
		for _, item := range defaultCategories() {
			if _, err := s.db.ExecContext(ctx, `
				insert into categories (id, main_category_id, name, kind, purpose, sort_order)
				values (?, ?, ?, ?, ?, ?)
			`, item.ID, item.MainID, item.Name, item.Kind, item.Purpose, item.SortOrder); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *SQLite) tableCount(ctx context.Context, table string) (int, error) {
	var count int
	err := s.db.QueryRowContext(ctx, fmt.Sprintf("select count(*) from %s", table)).Scan(&count)
	return count, err
}

func (s *SQLite) nextSortOrder(ctx context.Context, table string, where string, args ...any) (int, error) {
	query := fmt.Sprintf("select coalesce(max(sort_order), 0) + 10 from %s %s", table, where)
	var sortOrder int
	err := s.db.QueryRowContext(ctx, query, args...).Scan(&sortOrder)
	return sortOrder, err
}

func (s *SQLite) mainCategory(ctx context.Context, id int64) (MainCategory, error) {
	var item MainCategory
	err := s.db.QueryRowContext(ctx, `select id, name, sort_order from main_categories where id = ?`, id).Scan(&item.ID, &item.Name, &item.SortOrder)
	return item, err
}

func (s *SQLite) category(ctx context.Context, id int64) (Category, error) {
	var item Category
	err := s.db.QueryRowContext(ctx, `select id, coalesce(main_category_id, 0), name, kind, purpose, sort_order from categories where id = ?`, id).Scan(&item.ID, &item.MainID, &item.Name, &item.Kind, &item.Purpose, &item.SortOrder)
	return item, err
}

func (s *SQLite) account(ctx context.Context, id int64) (Account, error) {
	var item Account
	err := s.db.QueryRowContext(ctx, `
		select accounts.id, accounts.name, coalesce(accounts.iban, ''), accounts.account_type, coalesce(accounts.owner_name, ''), accounts.bank_profile, accounts.sort_order,
			accounts.opening_balance_cents + coalesce(sum(case when transactions.hidden = false then transactions.amount_cents else 0 end), 0) as balance_cents
		from accounts
		left join transactions on transactions.account_id = accounts.id
		where accounts.id = ?
		group by accounts.id
	`, id).Scan(&item.ID, &item.Name, &item.IBAN, &item.Type, &item.Owner, &item.BankProfile, &item.SortOrder, &item.BalanceCents)
	return item, err
}

type trimmedValue struct {
	value string
	err   error
}

func trimRequired(value string, name string) trimmedValue {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return trimmedValue{err: fmt.Errorf("%s is required", name)}
	}
	return trimmedValue{value: trimmed}
}

func defaultMainCategories() []MainCategory {
	return []MainCategory{
		{ID: 1, Name: "Inkomsten", SortOrder: 10},
		{ID: 2, Name: "Wonen", SortOrder: 20},
		{ID: 3, Name: "Huishouden", SortOrder: 30},
		{ID: 4, Name: "Medische kosten", SortOrder: 40},
		{ID: 5, Name: "Verzekeringen", SortOrder: 50},
		{ID: 6, Name: "Telecom", SortOrder: 60},
		{ID: 7, Name: "Vervoer", SortOrder: 70},
		{ID: 8, Name: "Educatie", SortOrder: 80},
		{ID: 9, Name: "Kleding", SortOrder: 90},
		{ID: 10, Name: "Vrije tijd", SortOrder: 100},
		{ID: 11, Name: "Overige uitgaven", SortOrder: 110},
	}
}

func defaultCategories() []Category {
	return []Category{
		{ID: 1, MainID: 3, Name: "Boodschappen", Kind: "expense", Purpose: "daily", SortOrder: 10},
		{ID: 2, MainID: 2, Name: "Wonen", Kind: "expense", Purpose: "daily", SortOrder: 20},
		{ID: 3, MainID: 10, Name: "Vakantie", Kind: "expense", Purpose: "reservation", SortOrder: 30},
		{ID: 4, MainID: 2, Name: "Onderhoud huis", Kind: "expense", Purpose: "reservation", SortOrder: 40},
		{ID: 5, MainID: 2, Name: "Verduurzaming woning", Kind: "expense", Purpose: "investment", SortOrder: 50},
		{ID: 6, MainID: 2, Name: "Verbouwing", Kind: "expense", Purpose: "investment", SortOrder: 60},
		{ID: 7, MainID: 1, Name: "Salaris", Kind: "income", Purpose: "daily", SortOrder: 70},
	}
}

const sqliteSchema = `
pragma foreign_keys = on;

create table if not exists households (
    id integer primary key,
    name text not null,
    created_at text not null default current_timestamp
);

create table if not exists accounts (
    id integer primary key,
    household_id integer not null default 1 references households(id) on delete cascade,
    name text not null,
    iban text,
    account_type text not null default 'checking' check (account_type in ('checking', 'savings')),
    owner_name text,
    bank_profile text not null default 'generic',
    currency text not null default 'EUR',
    opening_balance_cents integer not null default 0,
    sort_order integer not null default 100,
    created_at text not null default current_timestamp
);

create unique index if not exists accounts_iban_unique on accounts(iban) where iban is not null and iban != '';

create table if not exists main_categories (
    id integer primary key,
    name text not null unique,
    sort_order integer not null default 100,
    created_at text not null default current_timestamp
);

create table if not exists categories (
    id integer primary key,
    main_category_id integer references main_categories(id) on delete cascade,
    name text not null,
    kind text not null check (kind in ('income', 'expense', 'transfer')),
    purpose text not null default 'daily' check (purpose in ('daily', 'reservation', 'investment')),
    sort_order integer not null default 100,
    created_at text not null default current_timestamp,
    unique (main_category_id, name)
);

create index if not exists categories_main_purpose_idx on categories(main_category_id, purpose, sort_order);

create table if not exists transactions (
    id integer primary key,
    account_id integer not null references accounts(id) on delete cascade,
    category_id integer references categories(id) on delete set null,
    booked_on text not null,
    description text not null,
    counterparty text,
    counterparty_iban text,
    amount_cents integer not null,
    external_reference text,
    import_fingerprint text,
    hidden integer not null default 0,
    created_at text not null default current_timestamp
);

create index if not exists transactions_account_booked_on_idx on transactions(account_id, booked_on desc);
create index if not exists transactions_category_idx on transactions(category_id);
create unique index if not exists transactions_import_fingerprint_unique on transactions(import_fingerprint) where import_fingerprint is not null and import_fingerprint != '';
`
