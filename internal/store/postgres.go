package store

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Postgres struct {
	pool *pgxpool.Pool
}

func OpenPostgres(ctx context.Context, databaseURL string) (*Postgres, error) {
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, err
	}

	store := &Postgres{pool: pool}
	if err := store.Ping(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	if err := store.ensureSchema(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	if err := store.seedDefaults(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	return store, nil
}

func (s *Postgres) Close() {
	s.pool.Close()
}

func (s *Postgres) Ping(ctx context.Context) error {
	return s.pool.Ping(ctx)
}

func (s *Postgres) Summary(ctx context.Context) (Summary, error) {
	var summary Summary
	err := s.pool.QueryRow(ctx, `
		select
			(select count(*) from households),
			(select count(*) from accounts),
			(select count(*) from transactions where hidden = false)
	`).Scan(&summary.Households, &summary.Accounts, &summary.Transactions)
	return summary, err
}

func (s *Postgres) MainCategories(ctx context.Context) ([]MainCategory, error) {
	rows, err := s.pool.Query(ctx, `select id, name, sort_order from main_categories order by sort_order, name`)
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

func (s *Postgres) CreateMainCategory(ctx context.Context, input CreateMainCategory) (MainCategory, error) {
	name := trimRequired(input.Name, "name")
	if name.err != nil {
		return MainCategory{}, name.err
	}
	var sortOrder int
	if err := s.pool.QueryRow(ctx, `select coalesce(max(sort_order), 0) + 10 from main_categories`).Scan(&sortOrder); err != nil {
		return MainCategory{}, err
	}

	var item MainCategory
	err := s.pool.QueryRow(ctx, `
		insert into main_categories (name, sort_order)
		values ($1, $2)
		returning id, name, sort_order
	`, name.value, sortOrder).Scan(&item.ID, &item.Name, &item.SortOrder)
	return item, err
}

func (s *Postgres) UpdateMainCategory(ctx context.Context, id int64, input UpdateMainCategory) (MainCategory, error) {
	name := trimRequired(input.Name, "name")
	if name.err != nil {
		return MainCategory{}, name.err
	}
	var item MainCategory
	err := s.pool.QueryRow(ctx, `
		update main_categories set name = $1 where id = $2 returning id, name, sort_order
	`, name.value, id).Scan(&item.ID, &item.Name, &item.SortOrder)
	if err == pgx.ErrNoRows {
		return MainCategory{}, fmt.Errorf("main category not found")
	}
	return item, err
}

func (s *Postgres) DeleteMainCategory(ctx context.Context, id int64) error {
	var categoryCount int
	if err := s.pool.QueryRow(ctx, `select count(*) from categories where main_category_id = $1`, id).Scan(&categoryCount); err != nil {
		return err
	}
	if categoryCount > 0 {
		return fmt.Errorf("main category still has %d subcategories", categoryCount)
	}
	result, err := s.pool.Exec(ctx, `delete from main_categories where id = $1`, id)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return fmt.Errorf("main category not found")
	}
	return nil
}

func (s *Postgres) Categories(ctx context.Context) ([]Category, error) {
	rows, err := s.pool.Query(ctx, `
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

func (s *Postgres) CreateCategory(ctx context.Context, input CreateCategory) (Category, error) {
	name := trimRequired(input.Name, "name")
	if name.err != nil {
		return Category{}, name.err
	}
	kind, purpose, err := normalizeCategoryInput(input.Kind, input.Purpose)
	if err != nil {
		return Category{}, err
	}
	var sortOrder int
	if err := s.pool.QueryRow(ctx, `select coalesce(max(sort_order), 0) + 10 from categories where main_category_id = $1`, input.MainID).Scan(&sortOrder); err != nil {
		return Category{}, err
	}

	var item Category
	err = s.pool.QueryRow(ctx, `
		insert into categories (household_id, main_category_id, name, kind, purpose, sort_order)
		values ((select id from households order by id limit 1), $1, $2, $3, $4, $5)
		returning id, coalesce(main_category_id, 0), name, kind, purpose, sort_order
	`, input.MainID, name.value, kind, purpose, sortOrder).Scan(&item.ID, &item.MainID, &item.Name, &item.Kind, &item.Purpose, &item.SortOrder)
	return item, err
}

func (s *Postgres) UpdateCategory(ctx context.Context, id int64, input UpdateCategory) (Category, error) {
	name := trimRequired(input.Name, "name")
	if name.err != nil {
		return Category{}, name.err
	}
	kind, purpose, err := normalizeCategoryInput(input.Kind, input.Purpose)
	if err != nil {
		return Category{}, err
	}

	var item Category
	err = s.pool.QueryRow(ctx, `
		update categories
		set main_category_id = $1, name = $2, kind = $3, purpose = $4
		where id = $5
		returning id, coalesce(main_category_id, 0), name, kind, purpose, sort_order
	`, input.MainID, name.value, kind, purpose, id).Scan(&item.ID, &item.MainID, &item.Name, &item.Kind, &item.Purpose, &item.SortOrder)
	if err == pgx.ErrNoRows {
		return Category{}, fmt.Errorf("category not found")
	}
	return item, err
}

func (s *Postgres) DeleteCategory(ctx context.Context, id int64) error {
	var transactionCount int
	if err := s.pool.QueryRow(ctx, `select count(*) from transactions where category_id = $1`, id).Scan(&transactionCount); err != nil {
		return err
	}
	if transactionCount > 0 {
		return fmt.Errorf("category is used by %d transactions", transactionCount)
	}
	result, err := s.pool.Exec(ctx, `delete from categories where id = $1`, id)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return fmt.Errorf("category not found")
	}
	return nil
}

func (s *Postgres) Accounts(ctx context.Context) ([]Account, error) {
	rows, err := s.pool.Query(ctx, `
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

func (s *Postgres) CreateAccount(ctx context.Context, input SaveAccount) (Account, error) {
	account, err := normalizeAccountInput(input)
	if err != nil {
		return Account{}, err
	}
	if err := s.pool.QueryRow(ctx, `select coalesce(max(sort_order), 0) + 10 from accounts`).Scan(&account.SortOrder); err != nil {
		return Account{}, err
	}
	var id int64
	err = s.pool.QueryRow(ctx, `
		insert into accounts (household_id, name, iban, account_type, owner_name, bank_profile, sort_order)
		values ((select id from households order by id limit 1), $1, $2, $3, $4, $5, $6)
		returning id
	`, account.Name, account.IBAN, account.Type, account.Owner, account.BankProfile, account.SortOrder).Scan(&id)
	if err != nil {
		return Account{}, err
	}
	return s.account(ctx, id)
}

func (s *Postgres) UpdateAccount(ctx context.Context, id int64, input SaveAccount) (Account, error) {
	account, err := normalizeAccountInput(input)
	if err != nil {
		return Account{}, err
	}
	result, err := s.pool.Exec(ctx, `
		update accounts
		set name = $1, iban = $2, account_type = $3, owner_name = $4, bank_profile = $5
		where id = $6
	`, account.Name, account.IBAN, account.Type, account.Owner, account.BankProfile, id)
	if err != nil {
		return Account{}, err
	}
	if result.RowsAffected() == 0 {
		return Account{}, fmt.Errorf("account not found")
	}
	return s.account(ctx, id)
}

func (s *Postgres) DeleteAccount(ctx context.Context, id int64) error {
	var transactionCount int
	if err := s.pool.QueryRow(ctx, `select count(*) from transactions where account_id = $1`, id).Scan(&transactionCount); err != nil {
		return err
	}
	if transactionCount > 0 {
		return fmt.Errorf("account has %d transactions", transactionCount)
	}
	result, err := s.pool.Exec(ctx, `delete from accounts where id = $1`, id)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return fmt.Errorf("account not found")
	}
	return nil
}

func (s *Postgres) ImportTransactions(ctx context.Context, accountID int64, transactions []ImportedTransaction) (ImportResult, error) {
	result := ImportResult{Total: len(transactions)}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return ImportResult{}, err
	}
	defer tx.Rollback(ctx)

	occurrences := make(map[string]int)
	for _, transaction := range transactions {
		fingerprint := strings.TrimSpace(transaction.ImportFingerprint)
		if fingerprint == "" {
			baseFingerprint := transactionFingerprint(accountID, transaction)
			occurrences[baseFingerprint]++
			fingerprint = transactionFingerprintOccurrence(baseFingerprint, occurrences[baseFingerprint])
		}

		execResult, err := tx.Exec(ctx, `
			insert into transactions (
				account_id, booked_on, description, counterparty, counterparty_iban,
				amount_cents, external_reference, import_fingerprint
			) values ($1, $2, $3, $4, $5, $6, $7, $8)
			on conflict (import_fingerprint) where import_fingerprint is not null and import_fingerprint != '' do nothing
		`, accountID, transaction.BookedOn, transaction.Description, transaction.Counterparty, transaction.CounterpartyIBAN, transaction.AmountCents, transaction.ExternalReference, fingerprint)
		if err != nil {
			return ImportResult{}, err
		}
		if execResult.RowsAffected() == 0 {
			result.Duplicates++
			continue
		}
		result.Imported++
	}
	return result, tx.Commit(ctx)
}

func (s *Postgres) Transactions(ctx context.Context, filter TransactionFilter) ([]Transaction, error) {
	if filter.Limit <= 0 {
		filter.Limit = 25
	}
	if filter.Limit > 200 {
		filter.Limit = 200
	}
	if filter.Offset < 0 {
		filter.Offset = 0
	}
	search := strings.TrimSpace(filter.Search)
	rows, err := s.pool.Query(ctx, `
		select transactions.id, transactions.account_id, coalesce(transactions.category_id, 0), transactions.booked_on::text, transactions.description,
			coalesce(transactions.counterparty, ''), coalesce(transactions.counterparty_iban, ''), transactions.amount_cents,
			coalesce(transactions.external_reference, ''), coalesce(transactions.import_fingerprint, ''), transactions.hidden
		from transactions
		left join categories on categories.id = transactions.category_id
		where ($1::bigint = 0 or account_id = $2)
		and ($3::bigint = 0 or transactions.category_id = $4)
		and ($5::bigint = 0 or categories.main_category_id = $6)
		and ($7::text = '' or booked_on >= nullif($8, '')::date)
		and ($9::text = '' or booked_on <= nullif($10, '')::date)
		and (
			$11::text = ''
			or lower(transactions.description) like '%' || lower($12) || '%'
			or lower(coalesce(transactions.counterparty, '')) like '%' || lower($13) || '%'
			or lower(coalesce(transactions.counterparty_iban, '')) like '%' || lower($14) || '%'
			or lower(coalesce(transactions.external_reference, '')) like '%' || lower($15) || '%'
		)
		and ($16::boolean = false or transactions.category_id is null)
		and (($17::boolean = true and transactions.hidden = true) or ($18::boolean = false and transactions.hidden = false))
		order by transactions.booked_on desc, transactions.id desc
		limit $19 offset $20
	`, filter.AccountID, filter.AccountID, filter.CategoryID, filter.CategoryID, filter.MainCategoryID, filter.MainCategoryID, filter.From, filter.From, filter.To, filter.To, search, search, search, search, search, filter.Uncategorized, filter.HiddenOnly, filter.HiddenOnly, filter.Limit, filter.Offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := []Transaction{}
	for rows.Next() {
		var item Transaction
		if err := rows.Scan(&item.ID, &item.AccountID, &item.CategoryID, &item.BookedOn, &item.Description, &item.Counterparty, &item.CounterpartyIBAN, &item.AmountCents, &item.ExternalReference, &item.ImportFingerprint, &item.Hidden); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Postgres) TransactionYears(ctx context.Context) ([]int, error) {
	rows, err := s.pool.Query(ctx, `
		select distinct extract(year from booked_on)::integer as year
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

func (s *Postgres) TransactionSummary(ctx context.Context, filter TransactionFilter) (TransactionSummary, error) {
	search := strings.TrimSpace(filter.Search)
	var summary TransactionSummary
	err := s.pool.QueryRow(ctx, `
		select count(*), coalesce(sum(transactions.amount_cents), 0)
		from transactions
		left join categories on categories.id = transactions.category_id
		where ($1::bigint = 0 or account_id = $2)
		and ($3::bigint = 0 or transactions.category_id = $4)
		and ($5::bigint = 0 or categories.main_category_id = $6)
		and ($7::text = '' or booked_on >= nullif($8, '')::date)
		and ($9::text = '' or booked_on <= nullif($10, '')::date)
		and (
			$11::text = ''
			or lower(transactions.description) like '%' || lower($12) || '%'
			or lower(coalesce(transactions.counterparty, '')) like '%' || lower($13) || '%'
			or lower(coalesce(transactions.counterparty_iban, '')) like '%' || lower($14) || '%'
			or lower(coalesce(transactions.external_reference, '')) like '%' || lower($15) || '%'
		)
		and ($16::boolean = false or transactions.category_id is null)
		and (($17::boolean = true and transactions.hidden = true) or ($18::boolean = false and transactions.hidden = false))
	`, filter.AccountID, filter.AccountID, filter.CategoryID, filter.CategoryID, filter.MainCategoryID, filter.MainCategoryID, filter.From, filter.From, filter.To, filter.To, search, search, search, search, search, filter.Uncategorized, filter.HiddenOnly, filter.HiddenOnly).Scan(&summary.Count, &summary.AmountCents)
	return summary, err
}

func (s *Postgres) SetTransactionHidden(ctx context.Context, id int64, hidden bool) error {
	result, err := s.pool.Exec(ctx, `update transactions set hidden = $1 where id = $2`, hidden, id)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return fmt.Errorf("transaction not found")
	}
	return nil
}

func (s *Postgres) CategorizeTransaction(ctx context.Context, id int64, input CategorizeTransaction) (CategorizeResult, error) {
	if input.CategoryID == 0 {
		return CategorizeResult{}, fmt.Errorf("category is required")
	}
	if !input.ApplyToSameCounterparty {
		result, err := s.pool.Exec(ctx, `update transactions set category_id = $1 where id = $2`, input.CategoryID, id)
		if err != nil {
			return CategorizeResult{}, err
		}
		if result.RowsAffected() == 0 {
			return CategorizeResult{}, fmt.Errorf("transaction not found")
		}
		return CategorizeResult{Updated: int(result.RowsAffected())}, nil
	}

	var accountID int64
	var counterparty string
	if err := s.pool.QueryRow(ctx, `select account_id, coalesce(counterparty, '') from transactions where id = $1`, id).Scan(&accountID, &counterparty); err != nil {
		return CategorizeResult{}, fmt.Errorf("transaction not found")
	}
	if strings.TrimSpace(counterparty) == "" {
		result, err := s.pool.Exec(ctx, `update transactions set category_id = $1 where id = $2`, input.CategoryID, id)
		if err != nil {
			return CategorizeResult{}, err
		}
		return CategorizeResult{Updated: int(result.RowsAffected())}, nil
	}

	result, err := s.pool.Exec(ctx, `
		update transactions
		set category_id = $1
		where account_id = $2 and counterparty = $3 and category_id is null
	`, input.CategoryID, accountID, counterparty)
	if err != nil {
		return CategorizeResult{}, err
	}
	return CategorizeResult{Updated: int(result.RowsAffected())}, nil
}

func (s *Postgres) CategoryMonthlyTotals(ctx context.Context, filter CategoryReportFilter) ([]MonthlyTotal, error) {
	rows, err := s.pool.Query(ctx, `
		select to_char(transactions.booked_on, 'YYYY-MM') as month, sum(transactions.amount_cents)
		from transactions
		join categories on categories.id = transactions.category_id
		where ($1::bigint = 0 or transactions.category_id = $2)
		and ($3::bigint = 0 or categories.main_category_id = $4)
		and transactions.hidden = false
		group by month
		order by month
	`, filter.CategoryID, filter.CategoryID, filter.MainCategoryID, filter.MainCategoryID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := []MonthlyTotal{}
	for rows.Next() {
		var item MonthlyTotal
		if err := rows.Scan(&item.Month, &item.AmountCents); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Postgres) MainCategoryPeriodTotals(ctx context.Context, filter PeriodReportFilter) ([]CategoryTotal, error) {
	query := `
		select main_categories.id, main_categories.name, 0::bigint, '',
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
		and ($1::text = '' or transactions.booked_on >= nullif($2, '')::date)
		and ($3::text = '' or transactions.booked_on <= nullif($4, '')::date)
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
			and main_categories.id = $1
			and (
				(categories.kind = 'income' and transactions.amount_cents > 0)
				or (categories.kind = 'expense' and transactions.amount_cents < 0)
			)
			and ($2::text = '' or transactions.booked_on >= nullif($3, '')::date)
			and ($4::text = '' or transactions.booked_on <= nullif($5, '')::date)
			group by main_categories.id, main_categories.name, categories.id, categories.name, categories.sort_order
			order by amount_cents desc, categories.sort_order
		`
		args = []any{filter.MainCategoryID, filter.From, filter.From, filter.To, filter.To}
	}

	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := []CategoryTotal{}
	for rows.Next() {
		var item CategoryTotal
		if err := rows.Scan(&item.MainCategoryID, &item.MainCategoryName, &item.CategoryID, &item.CategoryName, &item.AmountCents); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Postgres) account(ctx context.Context, id int64) (Account, error) {
	var item Account
	err := s.pool.QueryRow(ctx, `
		select accounts.id, accounts.name, coalesce(accounts.iban, ''), accounts.account_type, coalesce(accounts.owner_name, ''), accounts.bank_profile, accounts.sort_order,
			accounts.opening_balance_cents + coalesce(sum(case when transactions.hidden = false then transactions.amount_cents else 0 end), 0) as balance_cents
		from accounts
		left join transactions on transactions.account_id = accounts.id
		where accounts.id = $1
		group by accounts.id
	`, id).Scan(&item.ID, &item.Name, &item.IBAN, &item.Type, &item.Owner, &item.BankProfile, &item.SortOrder, &item.BalanceCents)
	return item, err
}

func (s *Postgres) ensureSchema(ctx context.Context) error {
	_, err := s.pool.Exec(ctx, postgresSchema)
	return err
}

func (s *Postgres) seedDefaults(ctx context.Context) error {
	if _, err := s.pool.Exec(ctx, `insert into households (id, name) values (1, 'Huishouden') on conflict (id) do nothing`); err != nil {
		return err
	}
	var mainCategoryCount int
	if err := s.pool.QueryRow(ctx, `select count(*) from main_categories`).Scan(&mainCategoryCount); err != nil {
		return err
	}
	if mainCategoryCount == 0 {
		for _, item := range defaultMainCategories() {
			if _, err := s.pool.Exec(ctx, `insert into main_categories (id, name, sort_order) values ($1, $2, $3)`, item.ID, item.Name, item.SortOrder); err != nil {
				return err
			}
		}
	}
	var accountCount int
	if err := s.pool.QueryRow(ctx, `select count(*) from accounts`).Scan(&accountCount); err != nil {
		return err
	}
	if accountCount == 0 {
		if _, err := s.pool.Exec(ctx, `
			insert into accounts (id, household_id, name, iban, account_type, owner_name, bank_profile, sort_order)
			values (1, 1, 'Triodos betaalrekening', 'NL18TRIO0254363105', 'checking', 'Huishouden', 'triodos', 10)
		`); err != nil {
			return err
		}
	}
	var categoryCount int
	if err := s.pool.QueryRow(ctx, `select count(*) from categories`).Scan(&categoryCount); err != nil {
		return err
	}
	if categoryCount == 0 {
		for _, item := range defaultCategories() {
			if _, err := s.pool.Exec(ctx, `
				insert into categories (id, household_id, main_category_id, name, kind, purpose, sort_order)
				values ($1, 1, $2, $3, $4, $5, $6)
			`, item.ID, item.MainID, item.Name, item.Kind, item.Purpose, item.SortOrder); err != nil {
				return err
			}
		}
	}
	return s.updateSequences(ctx)
}

func (s *Postgres) updateSequences(ctx context.Context) error {
	for _, table := range []string{"households", "accounts", "main_categories", "categories", "transactions"} {
		if _, err := s.pool.Exec(ctx, fmt.Sprintf(`select setval(pg_get_serial_sequence('%s', 'id'), coalesce((select max(id) from %s), 1), true)`, table, table)); err != nil {
			return err
		}
	}
	return nil
}

const postgresSchema = `
create table if not exists households (
    id bigserial primary key,
    name text not null,
    created_at timestamptz not null default now()
);

insert into households (id, name) values (1, 'Huishouden') on conflict (id) do nothing;

create table if not exists accounts (
    id bigserial primary key,
    household_id bigint not null default 1 references households(id) on delete cascade,
    name text not null,
    iban text,
    account_type text not null default 'checking' check (account_type in ('checking', 'savings')),
    owner_name text,
    bank_profile text not null default 'generic',
    currency char(3) not null default 'EUR',
    opening_balance_cents bigint not null default 0,
    sort_order integer not null default 100,
    created_at timestamptz not null default now()
);

create unique index if not exists accounts_iban_unique on accounts(iban) where iban is not null and iban != '';

create table if not exists main_categories (
    id bigserial primary key,
    name text not null unique,
    sort_order integer not null default 100,
    created_at timestamptz not null default now()
);

create table if not exists categories (
    id bigserial primary key,
    household_id bigint not null default 1 references households(id) on delete cascade,
    main_category_id bigint references main_categories(id) on delete cascade,
    name text not null,
    kind text not null check (kind in ('income', 'expense', 'transfer')),
    purpose text not null default 'daily' check (purpose in ('daily', 'reservation', 'investment')),
    sort_order integer not null default 100,
    created_at timestamptz not null default now(),
    unique (main_category_id, name)
);

create index if not exists categories_main_purpose_idx on categories(main_category_id, purpose, sort_order);
create unique index if not exists categories_main_name_unique on categories(main_category_id, name);

create table if not exists transactions (
    id bigserial primary key,
    account_id bigint not null references accounts(id) on delete cascade,
    category_id bigint references categories(id) on delete set null,
    booked_on date not null,
    description text not null,
    counterparty text,
    counterparty_iban text,
    amount_cents bigint not null,
    external_reference text,
    import_fingerprint text,
    hidden boolean not null default false,
    created_at timestamptz not null default now()
);

alter table accounts add column if not exists household_id bigint not null default 1 references households(id) on delete cascade;
alter table accounts add column if not exists opening_balance_cents bigint not null default 0;
alter table categories add column if not exists household_id bigint not null default 1 references households(id) on delete cascade;
alter table transactions add column if not exists counterparty_iban text;
alter table transactions add column if not exists import_fingerprint text;
alter table transactions add column if not exists hidden boolean not null default false;

create index if not exists transactions_account_booked_on_idx on transactions(account_id, booked_on desc);
create index if not exists transactions_category_idx on transactions(category_id);
create unique index if not exists transactions_import_fingerprint_unique on transactions(import_fingerprint) where import_fingerprint is not null and import_fingerprint != '';
`
