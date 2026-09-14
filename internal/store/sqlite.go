package store

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"net/url"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

type SQLite struct {
	db *sql.DB
}

var (
	_ Backend                  = (*SQLite)(nil)
	_ DocumentRepository       = (*SQLite)(nil)
	_ AssetRepository          = (*SQLite)(nil)
	_ LiabilityRepository      = (*SQLite)(nil)
	_ CaseRepository           = (*SQLite)(nil)
	_ EntityRelationRepository = (*SQLite)(nil)
	_ DomainEventRepository    = (*SQLite)(nil)
)

func OpenSQLite(ctx context.Context, databasePath string) (*SQLite, error) {
	dsn, err := sqliteDSN(databasePath)
	if err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)

	store := &SQLite{db: db}
	if err := store.Ping(ctx); err != nil {
		db.Close()
		return nil, err
	}
	if err := store.initialize(ctx); err != nil {
		db.Close()
		return nil, err
	}
	return store, nil
}

func sqliteDSN(databasePath string) (string, error) {
	base, rawQuery, _ := strings.Cut(databasePath, "?")
	query, err := url.ParseQuery(rawQuery)
	if err != nil {
		return "", fmt.Errorf("invalid sqlite database path: %w", err)
	}
	query.Set("_foreign_keys", "on")
	query.Set("_busy_timeout", "5000")
	return base + "?" + query.Encode(), nil
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

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return MainCategory{}, err
	}
	defer tx.Rollback()
	var sortOrder int
	if err := tx.QueryRowContext(ctx, `select coalesce(max(sort_order),0)+10 from main_categories`).Scan(&sortOrder); err != nil {
		return MainCategory{}, err
	}
	result, err := tx.ExecContext(ctx, `insert into main_categories (name, sort_order) values (?, ?)`, name.value, sortOrder)
	if err != nil {
		return MainCategory{}, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return MainCategory{}, err
	}
	caseResult, err := tx.ExecContext(ctx, `insert into cases(household_id,main_category_id,case_type,title,status) values ((select id from households order by id limit 1),?,?,?,'active')`, id, CaseTypeMainCategory, name.value)
	if err != nil {
		return MainCategory{}, err
	}
	caseID, err := caseResult.LastInsertId()
	if err != nil {
		return MainCategory{}, err
	}
	if err := appendSQLiteDomainEvent(ctx, tx, AppendDomainEvent{EntityType: ObjectTypeCase, EntityID: caseID, EventType: "case_created", CurrentDate: true, Description: "Dossier aangemaakt"}); err != nil {
		return MainCategory{}, err
	}
	if err := tx.Commit(); err != nil {
		return MainCategory{}, err
	}
	return MainCategory{ID: id, Name: name.value, SortOrder: sortOrder}, nil
}

func (s *SQLite) UpdateMainCategory(ctx context.Context, id int64, input UpdateMainCategory) (MainCategory, error) {
	name := trimRequired(input.Name, "name")
	if name.err != nil {
		return MainCategory{}, name.err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return MainCategory{}, err
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, `update main_categories set name = ? where id = ?`, name.value, id)
	if err != nil {
		return MainCategory{}, err
	}
	if affected, _ := result.RowsAffected(); affected == 0 {
		return MainCategory{}, fmt.Errorf("main category not found")
	}
	var item MainCategory
	if err := tx.QueryRowContext(ctx, `select id,name,sort_order from main_categories where id=?`, id).Scan(&item.ID, &item.Name, &item.SortOrder); err != nil {
		return MainCategory{}, err
	}
	caseResult, err := tx.ExecContext(ctx, `update cases set title=?,updated_at=strftime('%Y-%m-%dT%H:%M:%fZ','now') where main_category_id=?`, item.Name, id)
	if err != nil {
		return MainCategory{}, err
	}
	if affected, _ := caseResult.RowsAffected(); affected != 1 {
		return MainCategory{}, fmt.Errorf("managed case not found")
	}
	var caseID int64
	if err := tx.QueryRowContext(ctx, `select id from cases where main_category_id=?`, id).Scan(&caseID); err != nil {
		return MainCategory{}, err
	}
	if err := appendSQLiteDomainEvent(ctx, tx, AppendDomainEvent{EntityType: ObjectTypeCase, EntityID: caseID, EventType: "case_updated", CurrentDate: true, Description: "Dossier bijgewerkt"}); err != nil {
		return MainCategory{}, err
	}
	if err := tx.Commit(); err != nil {
		return MainCategory{}, err
	}
	return item, nil
}

func (s *SQLite) DeleteMainCategory(ctx context.Context, id int64) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var categoryCount int
	if err := tx.QueryRowContext(ctx, `select count(*) from categories where main_category_id = ?`, id).Scan(&categoryCount); err != nil {
		return err
	}
	if categoryCount > 0 {
		return fmt.Errorf("main category still has %d subcategories", categoryCount)
	}
	var caseID int64
	if err := tx.QueryRowContext(ctx, `select id from cases where main_category_id=?`, id).Scan(&caseID); err == sql.ErrNoRows {
		return fmt.Errorf("main category not found")
	} else if err != nil {
		return err
	}
	var relationCount int
	if err := tx.QueryRowContext(ctx, `select count(*) from entity_relations where (source_type=? and source_id=?) or (target_type=? and target_id=?)`, ObjectTypeCase, caseID, ObjectTypeCase, caseID).Scan(&relationCount); err != nil {
		return err
	}
	if relationCount > 0 {
		return fmt.Errorf("managed case still has %d relations", relationCount)
	}
	if err := cleanupSQLiteDomainEvents(ctx, tx, ObjectTypeCase, caseID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `delete from cases where id=?`, caseID); err != nil {
		return err
	}
	result, err := tx.ExecContext(ctx, `delete from main_categories where id = ?`, id)
	if err != nil {
		return err
	}
	if affected, _ := result.RowsAffected(); affected == 0 {
		return fmt.Errorf("main category not found")
	}
	return tx.Commit()
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
		select accounts.id, accounts.name, coalesce(accounts.iban, ''), accounts.account_type, coalesce(accounts.owner_name, ''), accounts.bank_profile, accounts.currency, accounts.sort_order,
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
		if err := rows.Scan(&item.ID, &item.Name, &item.IBAN, &item.Type, &item.Owner, &item.BankProfile, &item.Currency, &item.SortOrder, &item.BalanceCents); err != nil {
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
	for _, transaction := range transactions {
		date, err := time.Parse(time.DateOnly, transaction.BookedOn)
		if err != nil || date.Format(time.DateOnly) != transaction.BookedOn {
			return ImportResult{}, fmt.Errorf("invalid booked date")
		}
	}
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
			insert into transactions (
				account_id, booked_on, description, counterparty, counterparty_iban,
				amount_cents, external_reference, import_fingerprint
			) values (?, ?, ?, ?, ?, ?, ?, ?)
			on conflict (import_fingerprint) where import_fingerprint is not null and import_fingerprint != '' do nothing
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
			coalesce(transactions.external_reference, ''), coalesce(transactions.import_fingerprint, ''), transactions.hidden,
			(select count(distinct source_id) from entity_relations where source_type = 'document' and target_type = 'transaction' and target_id = transactions.id and relation_type = 'evidence_for')
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
			&transaction.DocumentCount,
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

type sqliteInitializer interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func (s *SQLite) initialize(ctx context.Context) error {
	conn, err := s.db.Conn(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()
	if _, err := conn.ExecContext(ctx, `begin immediate`); err != nil {
		return err
	}
	committed := false
	defer func() {
		if !committed {
			_, _ = conn.ExecContext(context.Background(), `rollback`)
		}
	}()
	if err := s.ensureSchema(ctx, conn); err != nil {
		return err
	}
	if err := s.seedDefaults(ctx, conn); err != nil {
		return err
	}
	if _, err := conn.ExecContext(ctx, `commit`); err != nil {
		return err
	}
	committed = true
	return nil
}

func (s *SQLite) ensureSchema(ctx context.Context, db sqliteInitializer) error {
	if _, err := db.ExecContext(ctx, sqliteSchema); err != nil {
		return err
	}
	if err := s.ensureColumn(ctx, db, "transactions", "hidden", "integer not null default 0 check (hidden in (0, 1))"); err != nil {
		return err
	}
	if err := s.ensureColumn(ctx, db, "cases", "main_category_id", "integer references main_categories(id) on delete cascade"); err != nil {
		return err
	}
	if _, err := db.ExecContext(ctx, sqliteUpgradeSchema); err != nil {
		return err
	}
	var foreignKeys int
	if err := db.QueryRowContext(ctx, `pragma foreign_keys`).Scan(&foreignKeys); err != nil {
		return err
	}
	if foreignKeys != 1 {
		return fmt.Errorf("sqlite foreign keys are disabled")
	}
	var busyTimeout int
	if err := db.QueryRowContext(ctx, `pragma busy_timeout`).Scan(&busyTimeout); err != nil {
		return err
	}
	if busyTimeout != 5000 {
		return fmt.Errorf("sqlite busy timeout is disabled")
	}
	return nil
}

func (s *SQLite) ensureColumn(ctx context.Context, db sqliteInitializer, table string, column string, definition string) error {
	rows, err := db.QueryContext(ctx, fmt.Sprintf("pragma table_info(%s)", table))
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
	_, err = db.ExecContext(ctx, fmt.Sprintf("alter table %s add column %s %s", table, column, definition))
	return err
}

func (s *SQLite) seedDefaults(ctx context.Context, db sqliteInitializer) error {
	if _, err := db.ExecContext(ctx, `insert into households (id, name) values (1, 'Huishouden') on conflict (id) do nothing`); err != nil {
		return err
	}
	mainCategoryCount, err := s.tableCount(ctx, db, "main_categories")
	if err != nil {
		return err
	}
	if mainCategoryCount == 0 {
		for _, item := range defaultMainCategories() {
			if _, err := db.ExecContext(ctx, `insert into main_categories (id, name, sort_order) values (?, ?, ?)`, item.ID, item.Name, item.SortOrder); err != nil {
				return err
			}
		}
	}
	accountCount, err := s.tableCount(ctx, db, "accounts")
	if err != nil {
		return err
	}
	if accountCount == 0 {
		if _, err := db.ExecContext(ctx, `
			insert into accounts (id, household_id, name, iban, account_type, owner_name, bank_profile, sort_order)
			values (1, 1, 'Triodos betaalrekening', 'NL18TRIO0254363105', 'checking', 'Huishouden', 'triodos', 10)
		`); err != nil {
			return err
		}
	}
	categoryCount, err := s.tableCount(ctx, db, "categories")
	if err != nil {
		return err
	}
	if categoryCount == 0 {
		for _, item := range defaultCategories() {
			if _, err := db.ExecContext(ctx, `
				insert into categories (id, main_category_id, name, kind, purpose, sort_order)
				values (?, ?, ?, ?, ?, ?)
			`, item.ID, item.MainID, item.Name, item.Kind, item.Purpose, item.SortOrder); err != nil {
				return err
			}
		}
	}
	if _, err := db.ExecContext(ctx, `
		insert into cases (household_id, main_category_id, case_type, title, status, created_at, updated_at)
		select (select id from households order by id limit 1), main_categories.id, 'main_category', main_categories.name, 'active', main_categories.created_at, main_categories.created_at
		from main_categories where not exists (select 1 from cases where cases.main_category_id = main_categories.id)
	`); err != nil {
		return err
	}
	if _, err := db.ExecContext(ctx, `
		insert into domain_events (entity_type, entity_id, event_type, event_date, description, created_at)
		select 'case', cases.id, 'case_created', substr(cases.created_at, 1, 10), 'Dossier aangemaakt', cases.created_at
		from cases where cases.main_category_id is not null on conflict do nothing
	`); err != nil {
		return err
	}
	return nil
}

func (s *SQLite) tableCount(ctx context.Context, db sqliteInitializer, table string) (int, error) {
	var count int
	err := db.QueryRowContext(ctx, fmt.Sprintf("select count(*) from %s", table)).Scan(&count)
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
		select accounts.id, accounts.name, coalesce(accounts.iban, ''), accounts.account_type, coalesce(accounts.owner_name, ''), accounts.bank_profile, accounts.currency, accounts.sort_order,
			accounts.opening_balance_cents + coalesce(sum(case when transactions.hidden = false then transactions.amount_cents else 0 end), 0) as balance_cents
		from accounts
		left join transactions on transactions.account_id = accounts.id
		where accounts.id = ?
		group by accounts.id
	`, id).Scan(&item.ID, &item.Name, &item.IBAN, &item.Type, &item.Owner, &item.BankProfile, &item.Currency, &item.SortOrder, &item.BalanceCents)
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
    created_at text not null default (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
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
    created_at text not null default (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
);

create unique index if not exists accounts_iban_unique on accounts(iban) where iban is not null and iban != '';

create table if not exists main_categories (
    id integer primary key,
    name text not null unique,
    sort_order integer not null default 100,
    created_at text not null default (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
);

create table if not exists categories (
    id integer primary key,
    main_category_id integer references main_categories(id) on delete cascade,
    name text not null,
    kind text not null check (kind in ('income', 'expense', 'transfer')),
    purpose text not null default 'daily' check (purpose in ('daily', 'reservation', 'investment')),
    sort_order integer not null default 100,
    created_at text not null default (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    unique (main_category_id, name)
);

create index if not exists categories_main_purpose_idx on categories(main_category_id, purpose, sort_order);

create table if not exists transactions (
    id integer primary key,
    account_id integer not null references accounts(id) on delete cascade,
    category_id integer references categories(id) on delete set null,
    booked_on text not null check (length(booked_on) = 10 and strftime('%Y-%m-%d', booked_on) = booked_on),
    description text not null,
    counterparty text,
    counterparty_iban text,
    amount_cents integer not null,
    external_reference text,
    import_fingerprint text,
    hidden integer not null default 0 check (hidden in (0, 1)),
    created_at text not null default (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
);

create index if not exists transactions_account_booked_on_idx on transactions(account_id, booked_on desc);
create index if not exists transactions_category_idx on transactions(category_id);
create unique index if not exists transactions_import_fingerprint_unique on transactions(import_fingerprint) where import_fingerprint is not null and import_fingerprint != '';

create table if not exists documents (
    id integer primary key,
    original_filename text not null,
    mime_type text not null,
    size_bytes integer not null check (size_bytes >= 0),
    sha256 text not null check (length(sha256) = 64),
    object_key text not null unique,
    document_date text check (document_date is null or (length(document_date) = 10 and strftime('%Y-%m-%d', document_date) = document_date)),
    description text,
    created_at text not null default (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
);
create index if not exists documents_sha256_idx on documents(sha256);

create table if not exists assets (
    id integer primary key,
    household_id integer not null default 1 references households(id) on delete cascade,
    asset_type text not null check (asset_type in ('real_estate', 'bank_account', 'investment_account', 'vehicle', 'equipment', 'valuable', 'cash', 'other')),
    name text not null,
    description text,
    status text not null default 'active' check (status in ('planned', 'active', 'disposed')),
    acquired_at text check (acquired_at is null or (length(acquired_at) = 10 and strftime('%Y-%m-%d', acquired_at) = acquired_at)),
    disposed_at text check (disposed_at is null or (length(disposed_at) = 10 and strftime('%Y-%m-%d', disposed_at) = disposed_at)),
    purchase_value integer not null default 0 check (purchase_value >= 0),
    current_value integer not null default 0 check (current_value >= 0),
    currency text not null default 'EUR' check (length(currency) = 3),
    created_at text not null default (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    updated_at text not null default (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
);
create index if not exists assets_household_type_status_idx on assets(household_id, asset_type, status, name);

create table if not exists asset_valuations (
    id integer primary key,
    asset_id integer not null references assets(id) on delete cascade,
    valuation_date text not null check (length(valuation_date) = 10 and strftime('%Y-%m-%d', valuation_date) = valuation_date),
    value integer not null check (value >= 0),
    valuation_type text not null check (valuation_type in ('purchase', 'estimate', 'appraisal', 'tax', 'market', 'other')),
    source text,
    notes text,
    created_at text not null default (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
);
create index if not exists asset_valuations_asset_date_idx on asset_valuations(asset_id, valuation_date desc, id desc);

create table if not exists liabilities (
    id integer primary key,
    household_id integer not null default 1 references households(id) on delete cascade,
    liability_type text not null check (liability_type in ('mortgage', 'loan', 'tax_debt', 'credit', 'other')),
    name text not null,
    description text,
    creditor text,
    start_date text check (start_date is null or (length(start_date) = 10 and strftime('%Y-%m-%d', start_date) = start_date)),
    end_date text check (end_date is null or (length(end_date) = 10 and strftime('%Y-%m-%d', end_date) = end_date)),
    original_amount integer not null default 0 check (original_amount >= 0),
    current_balance integer not null default 0 check (current_balance >= 0),
    currency text not null default 'EUR' check (length(currency) = 3),
    status text not null default 'active' check (status in ('planned', 'active', 'repaid', 'cancelled')),
    created_at text not null default (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    updated_at text not null default (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    check (end_date is null or start_date is null or end_date >= start_date)
);
create index if not exists liabilities_household_type_status_idx on liabilities(household_id, liability_type, status, name);

create table if not exists cases (
    id integer primary key,
    household_id integer not null default 1 references households(id) on delete cascade,
    main_category_id integer references main_categories(id) on delete cascade,
    case_type text not null check (trim(case_type) <> ''),
    title text not null check (trim(title) <> ''),
    description text,
    start_date text check (start_date is null or (length(start_date) = 10 and strftime('%Y-%m-%d', start_date) = start_date)),
    end_date text check (end_date is null or (length(end_date) = 10 and strftime('%Y-%m-%d', end_date) = end_date)),
    status text not null default 'active' check (status in ('planned', 'active', 'completed', 'archived')),
    created_at text not null default (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    updated_at text not null default (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    check (end_date is null or start_date is null or end_date >= start_date)
);
create index if not exists cases_household_type_status_title_idx on cases(household_id, case_type, status, title);

create table if not exists entity_relations (
    id integer primary key,
    source_type text not null,
    source_id integer not null check (source_id > 0),
    target_type text not null,
    target_id integer not null check (target_id > 0),
    relation_type text not null check (relation_type in ('relates_to', 'evidence_for', 'financed_by')),
    valid_from text check (valid_from is null or (length(valid_from) = 10 and strftime('%Y-%m-%d', valid_from) = valid_from)),
    valid_to text check (valid_to is null or (length(valid_to) = 10 and strftime('%Y-%m-%d', valid_to) = valid_to)),
    notes text,
    created_at text not null default (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    check (source_type <> target_type or source_id <> target_id),
    check (valid_to is null or valid_from is null or valid_to >= valid_from)
);
create unique index if not exists entity_relations_unique_idx on entity_relations(source_type, source_id, target_type, target_id, relation_type, coalesce(valid_from, ''), coalesce(valid_to, ''));
create index if not exists entity_relations_source_idx on entity_relations(source_type, source_id, relation_type);
create index if not exists entity_relations_target_idx on entity_relations(target_type, target_id, relation_type);

create table if not exists domain_events (
    id integer primary key,
    entity_type text not null check (entity_type in ('asset', 'liability', 'case')),
    entity_id integer not null check (entity_id > 0),
    event_type text not null check (trim(event_type) <> ''),
    event_date text not null check (length(event_date) = 10 and strftime('%Y-%m-%d', event_date) = event_date),
    description text not null check (trim(description) <> ''),
    related_entity_type text check (related_entity_type is null or related_entity_type in ('document', 'transaction', 'asset', 'liability', 'case')),
    related_entity_id integer check (related_entity_id is null or related_entity_id > 0),
    created_at text not null default (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    check ((related_entity_type is null) = (related_entity_id is null)),
    check (related_entity_type is null or entity_type <> related_entity_type or entity_id <> related_entity_id)
);
create index if not exists domain_events_timeline_idx on domain_events(entity_type, entity_id, event_date desc, id desc);
create unique index if not exists domain_events_creation_unique_idx on domain_events(entity_type, entity_id) where event_type in ('asset_created', 'liability_created', 'case_created');

insert into domain_events (entity_type, entity_id, event_type, event_date, description, created_at)
select 'asset', id, 'asset_created', substr(created_at, 1, 10), 'Bezitting aangemaakt', created_at from assets where true on conflict do nothing;
insert into domain_events (entity_type, entity_id, event_type, event_date, description, created_at)
select 'liability', id, 'liability_created', substr(created_at, 1, 10), 'Schuld aangemaakt', created_at from liabilities where true on conflict do nothing;
insert into domain_events (entity_type, entity_id, event_type, event_date, description, created_at)
select 'case', id, 'case_created', substr(created_at, 1, 10), 'Dossier aangemaakt', created_at from cases where true on conflict do nothing;
`

const sqliteUpgradeSchema = `
create unique index if not exists cases_main_category_unique_idx on cases(main_category_id) where main_category_id is not null;

create trigger if not exists accounts_type_insert_check
before insert on accounts
when new.account_type not in ('checking', 'savings')
begin
    select raise(abort, 'invalid account_type');
end;

create trigger if not exists accounts_type_update_check
before update of account_type on accounts
when new.account_type not in ('checking', 'savings')
begin
    select raise(abort, 'invalid account_type');
end;

create trigger if not exists categories_kind_purpose_insert_check
before insert on categories
when new.kind not in ('income', 'expense', 'transfer') or new.purpose not in ('daily', 'reservation', 'investment')
begin
    select raise(abort, 'invalid category kind or purpose');
end;

create trigger if not exists categories_kind_purpose_update_check
before update of kind, purpose on categories
when new.kind not in ('income', 'expense', 'transfer') or new.purpose not in ('daily', 'reservation', 'investment')
begin
    select raise(abort, 'invalid category kind or purpose');
end;

create trigger if not exists transactions_booked_on_insert_check
before insert on transactions
when new.booked_on is null or length(new.booked_on) != 10 or new.booked_on != date(new.booked_on, '+0 days')
begin
    select raise(abort, 'invalid booked_on');
end;

create trigger if not exists transactions_booked_on_update_check
before update of booked_on on transactions
when new.booked_on is null or length(new.booked_on) != 10 or new.booked_on != date(new.booked_on, '+0 days')
begin
    select raise(abort, 'invalid booked_on');
end;

create trigger if not exists transactions_hidden_insert_check
before insert on transactions
when new.hidden is null or new.hidden not in (0, 1)
begin
    select raise(abort, 'invalid hidden');
end;

create trigger if not exists transactions_hidden_update_check
before update of hidden on transactions
when new.hidden is null or new.hidden not in (0, 1)
begin
    select raise(abort, 'invalid hidden');
end;
`
