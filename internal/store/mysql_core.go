package store

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

func (s *MySQL) Summary(ctx context.Context) (Summary, error) {
	var v Summary
	err := s.db.QueryRowContext(ctx, `select (select count(*) from households),(select count(*) from accounts),(select count(*) from transactions where hidden=false)`).Scan(&v.Households, &v.Accounts, &v.Transactions)
	return v, err
}

func (s *MySQL) MainCategories(ctx context.Context) ([]MainCategory, error) {
	rows, err := s.db.QueryContext(ctx, `select id,name,sort_order from main_categories order by sort_order,name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []MainCategory{}
	for rows.Next() {
		var v MainCategory
		if err := rows.Scan(&v.ID, &v.Name, &v.SortOrder); err != nil {
			return nil, err
		}
		items = append(items, v)
	}
	return items, rows.Err()
}

func (s *MySQL) CreateMainCategory(ctx context.Context, input CreateMainCategory) (MainCategory, error) {
	name := trimRequired(input.Name, "name")
	if name.err != nil {
		return MainCategory{}, name.err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return MainCategory{}, err
	}
	defer tx.Rollback()
	var order int
	if err := tx.QueryRowContext(ctx, `select coalesce(max(sort_order),0)+10 from main_categories`).Scan(&order); err != nil {
		return MainCategory{}, err
	}
	r, err := tx.ExecContext(ctx, `insert into main_categories(name,sort_order) values (?,?)`, name.value, order)
	if err != nil {
		return MainCategory{}, err
	}
	id, err := r.LastInsertId()
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
	if err := mysqlAppendDomainEvent(ctx, tx, AppendDomainEvent{EntityType: ObjectTypeCase, EntityID: caseID, EventType: "case_created", CurrentDate: true, Description: "Dossier aangemaakt"}); err != nil {
		return MainCategory{}, err
	}
	if err := tx.Commit(); err != nil {
		return MainCategory{}, err
	}
	return MainCategory{ID: id, Name: name.value, SortOrder: order}, nil
}

func (s *MySQL) UpdateMainCategory(ctx context.Context, id int64, input UpdateMainCategory) (MainCategory, error) {
	name := trimRequired(input.Name, "name")
	if name.err != nil {
		return MainCategory{}, name.err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return MainCategory{}, err
	}
	defer tx.Rollback()
	r, err := tx.ExecContext(ctx, `update main_categories set name=? where id=?`, name.value, id)
	if err != nil {
		return MainCategory{}, err
	}
	n, _ := r.RowsAffected()
	if n == 0 {
		return MainCategory{}, fmt.Errorf("main category not found")
	}
	var v MainCategory
	if err = tx.QueryRowContext(ctx, `select id,name,sort_order from main_categories where id=?`, id).Scan(&v.ID, &v.Name, &v.SortOrder); err != nil {
		return MainCategory{}, err
	}
	caseResult, err := tx.ExecContext(ctx, `update cases set title=?,updated_at=utc_timestamp(6) where main_category_id=?`, v.Name, id)
	if err != nil {
		return MainCategory{}, err
	}
	if affected, _ := caseResult.RowsAffected(); affected != 1 {
		return MainCategory{}, fmt.Errorf("managed case not found")
	}
	var caseID int64
	if err = tx.QueryRowContext(ctx, `select id from cases where main_category_id=?`, id).Scan(&caseID); err != nil {
		return MainCategory{}, err
	}
	if err = mysqlAppendDomainEvent(ctx, tx, AppendDomainEvent{EntityType: ObjectTypeCase, EntityID: caseID, EventType: "case_updated", CurrentDate: true, Description: "Dossier bijgewerkt"}); err != nil {
		return MainCategory{}, err
	}
	if err = tx.Commit(); err != nil {
		return MainCategory{}, err
	}
	return v, nil
}

func (s *MySQL) DeleteMainCategory(ctx context.Context, id int64) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var n int
	if err := tx.QueryRowContext(ctx, `select count(*) from categories where main_category_id=?`, id).Scan(&n); err != nil {
		return err
	}
	if n > 0 {
		return fmt.Errorf("main category still has %d subcategories", n)
	}
	var caseID int64
	if err := tx.QueryRowContext(ctx, `select id from cases where main_category_id=? for update`, id).Scan(&caseID); err == sql.ErrNoRows {
		return fmt.Errorf("main category not found")
	} else if err != nil {
		return err
	}
	if err := tx.QueryRowContext(ctx, `select count(*) from entity_relations where (source_type=? and source_id=?) or (target_type=? and target_id=?)`, ObjectTypeCase, caseID, ObjectTypeCase, caseID).Scan(&n); err != nil {
		return err
	}
	if n > 0 {
		return fmt.Errorf("managed case still has %d relations", n)
	}
	if err := mysqlCleanupDomainEvents(ctx, tx, ObjectTypeCase, caseID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `delete from cases where id=?`, caseID); err != nil {
		return err
	}
	r, err := tx.ExecContext(ctx, `delete from main_categories where id=?`, id)
	if err := mysqlDelete(r, err, "main category not found"); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *MySQL) Categories(ctx context.Context) ([]Category, error) {
	rows, err := s.db.QueryContext(ctx, `select id,coalesce(main_category_id,0),name,kind,purpose,sort_order from categories order by sort_order,name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []Category{}
	for rows.Next() {
		var v Category
		if err := rows.Scan(&v.ID, &v.MainID, &v.Name, &v.Kind, &v.Purpose, &v.SortOrder); err != nil {
			return nil, err
		}
		items = append(items, v)
	}
	return items, rows.Err()
}

func (s *MySQL) CreateCategory(ctx context.Context, input CreateCategory) (Category, error) {
	name := trimRequired(input.Name, "name")
	if name.err != nil {
		return Category{}, name.err
	}
	kind, purpose, err := normalizeCategoryInput(input.Kind, input.Purpose)
	if err != nil {
		return Category{}, err
	}
	var order int
	if err = s.db.QueryRowContext(ctx, `select coalesce(max(sort_order),0)+10 from categories where main_category_id=?`, input.MainID).Scan(&order); err != nil {
		return Category{}, err
	}
	r, err := s.db.ExecContext(ctx, `insert into categories(household_id,main_category_id,name,kind,purpose,sort_order) values ((select id from households order by id limit 1),?,?,?,?,?)`, input.MainID, name.value, kind, purpose, order)
	if err != nil {
		return Category{}, err
	}
	id, err := r.LastInsertId()
	return Category{ID: id, MainID: input.MainID, Name: name.value, Kind: kind, Purpose: purpose, SortOrder: order}, err
}

func (s *MySQL) UpdateCategory(ctx context.Context, id int64, input UpdateCategory) (Category, error) {
	name := trimRequired(input.Name, "name")
	if name.err != nil {
		return Category{}, name.err
	}
	kind, purpose, err := normalizeCategoryInput(input.Kind, input.Purpose)
	if err != nil {
		return Category{}, err
	}
	r, err := s.db.ExecContext(ctx, `update categories set main_category_id=?,name=?,kind=?,purpose=? where id=?`, input.MainID, name.value, kind, purpose, id)
	if err != nil {
		return Category{}, err
	}
	n, _ := r.RowsAffected()
	if n == 0 {
		return Category{}, fmt.Errorf("category not found")
	}
	var v Category
	err = s.db.QueryRowContext(ctx, `select id,coalesce(main_category_id,0),name,kind,purpose,sort_order from categories where id=?`, id).Scan(&v.ID, &v.MainID, &v.Name, &v.Kind, &v.Purpose, &v.SortOrder)
	return v, err
}

func (s *MySQL) DeleteCategory(ctx context.Context, id int64) error {
	var n int
	if err := s.db.QueryRowContext(ctx, `select count(*) from transactions where category_id=?`, id).Scan(&n); err != nil {
		return err
	}
	if n > 0 {
		return fmt.Errorf("category is used by %d transactions", n)
	}
	r, err := s.db.ExecContext(ctx, `delete from categories where id=?`, id)
	return mysqlDelete(r, err, "category not found")
}

const mysqlAccountSelect = `select accounts.id,accounts.name,coalesce(accounts.iban,''),accounts.account_type,coalesce(accounts.owner_name,''),accounts.bank_profile,accounts.currency,accounts.sort_order,accounts.opening_balance_cents+coalesce(sum(case when transactions.hidden=false then transactions.amount_cents else 0 end),0) from accounts left join transactions on transactions.account_id=accounts.id`

func (s *MySQL) Accounts(ctx context.Context) ([]Account, error) {
	rows, err := s.db.QueryContext(ctx, mysqlAccountSelect+` group by accounts.id order by accounts.sort_order,accounts.name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []Account{}
	for rows.Next() {
		var v Account
		if err := rows.Scan(&v.ID, &v.Name, &v.IBAN, &v.Type, &v.Owner, &v.BankProfile, &v.Currency, &v.SortOrder, &v.BalanceCents); err != nil {
			return nil, err
		}
		items = append(items, v)
	}
	return items, rows.Err()
}

func (s *MySQL) account(ctx context.Context, id int64) (Account, error) {
	var v Account
	err := s.db.QueryRowContext(ctx, mysqlAccountSelect+` where accounts.id=? group by accounts.id`, id).Scan(&v.ID, &v.Name, &v.IBAN, &v.Type, &v.Owner, &v.BankProfile, &v.Currency, &v.SortOrder, &v.BalanceCents)
	return v, err
}

func (s *MySQL) CreateAccount(ctx context.Context, input SaveAccount) (Account, error) {
	v, err := normalizeAccountInput(input)
	if err != nil {
		return Account{}, err
	}
	if err = s.db.QueryRowContext(ctx, `select coalesce(max(sort_order),0)+10 from accounts`).Scan(&v.SortOrder); err != nil {
		return Account{}, err
	}
	r, err := s.db.ExecContext(ctx, `insert into accounts(household_id,name,iban,account_type,owner_name,bank_profile,sort_order) values ((select id from households order by id limit 1),?,?,?,?,?,?)`, v.Name, v.IBAN, v.Type, v.Owner, v.BankProfile, v.SortOrder)
	if err != nil {
		return Account{}, err
	}
	id, err := r.LastInsertId()
	if err != nil {
		return Account{}, err
	}
	return s.account(ctx, id)
}

func (s *MySQL) UpdateAccount(ctx context.Context, id int64, input SaveAccount) (Account, error) {
	v, err := normalizeAccountInput(input)
	if err != nil {
		return Account{}, err
	}
	r, err := s.db.ExecContext(ctx, `update accounts set name=?,iban=?,account_type=?,owner_name=?,bank_profile=? where id=?`, v.Name, v.IBAN, v.Type, v.Owner, v.BankProfile, id)
	if err != nil {
		return Account{}, err
	}
	n, _ := r.RowsAffected()
	if n == 0 {
		return Account{}, fmt.Errorf("account not found")
	}
	return s.account(ctx, id)
}

func (s *MySQL) DeleteAccount(ctx context.Context, id int64) error {
	var n int
	if err := s.db.QueryRowContext(ctx, `select count(*) from transactions where account_id=?`, id).Scan(&n); err != nil {
		return err
	}
	if n > 0 {
		return fmt.Errorf("account has %d transactions", n)
	}
	r, err := s.db.ExecContext(ctx, `delete from accounts where id=?`, id)
	return mysqlDelete(r, err, "account not found")
}

func mysqlDelete(result sql.Result, err error, message string) error {
	if err != nil {
		return err
	}
	n, e := result.RowsAffected()
	if e != nil {
		return e
	}
	if n == 0 {
		return fmt.Errorf("%s", message)
	}
	return nil
}

func (s *MySQL) ImportTransactions(ctx context.Context, accountID int64, transactions []ImportedTransaction) (ImportResult, error) {
	out := ImportResult{Total: len(transactions)}
	for i := range transactions {
		bookedOn, err := mysqlCanonicalDate(transactions[i].BookedOn, false)
		if err != nil {
			return ImportResult{}, fmt.Errorf("invalid booked date")
		}
		transactions[i].BookedOn = bookedOn
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return ImportResult{}, err
	}
	defer tx.Rollback()
	occurrences := map[string]int{}
	for _, v := range transactions {
		fingerprint := strings.TrimSpace(v.ImportFingerprint)
		if fingerprint == "" {
			base := transactionFingerprint(accountID, v)
			occurrences[base]++
			fingerprint = transactionFingerprintOccurrence(base, occurrences[base])
		}
		_, err = tx.ExecContext(ctx, `insert into transactions(account_id,booked_on,description,counterparty,counterparty_iban,amount_cents,external_reference,import_fingerprint) values (?,?,?,?,?,?,?,?)`, accountID, v.BookedOn, v.Description, v.Counterparty, v.CounterpartyIBAN, v.AmountCents, v.ExternalReference, fingerprint)
		if mysqlDuplicate(err) {
			out.Duplicates++
			continue
		}
		if err != nil {
			return ImportResult{}, err
		}
		out.Imported++
	}
	return out, tx.Commit()
}

func mysqlTransactionWhere(filter TransactionFilter) (string, []any) {
	search := strings.TrimSpace(filter.Search)
	where := ` where (?=0 or transactions.account_id=?) and (?=0 or transactions.category_id=?) and (?=0 or categories.main_category_id=?) and (?='' or booked_on>=?) and (?='' or booked_on<=?) and (?='' or lower(transactions.description) like concat('%',lower(?),'%') or lower(coalesce(transactions.counterparty,'')) like concat('%',lower(?),'%') or lower(coalesce(transactions.counterparty_iban,'')) like concat('%',lower(?),'%') or lower(coalesce(transactions.external_reference,'')) like concat('%',lower(?),'%')) and (?=false or transactions.category_id is null) and ((?=true and transactions.hidden=true) or (?=false and transactions.hidden=false))`
	return where, []any{filter.AccountID, filter.AccountID, filter.CategoryID, filter.CategoryID, filter.MainCategoryID, filter.MainCategoryID, filter.From, filter.From, filter.To, filter.To, search, search, search, search, search, filter.Uncategorized, filter.HiddenOnly, filter.HiddenOnly}
}

func mysqlCanonicalDate(value string, optional bool) (string, error) {
	if optional && value == "" {
		return "", nil
	}
	if len(value) != len("2006-01-02") {
		return "", fmt.Errorf("date must use YYYY-MM-DD")
	}
	parsed, err := time.Parse("2006-01-02", value)
	if err != nil || parsed.Format("2006-01-02") != value {
		return "", fmt.Errorf("date must use YYYY-MM-DD")
	}
	return value, nil
}

func mysqlValidateTransactionFilter(filter TransactionFilter) error {
	from, err := mysqlCanonicalDate(filter.From, true)
	if err != nil {
		return fmt.Errorf("invalid from date")
	}
	to, err := mysqlCanonicalDate(filter.To, true)
	if err != nil {
		return fmt.Errorf("invalid to date")
	}
	if from != "" && to != "" && from > to {
		return fmt.Errorf("to date cannot be before from date")
	}
	return nil
}

func (s *MySQL) Transactions(ctx context.Context, filter TransactionFilter) ([]Transaction, error) {
	if err := mysqlValidateTransactionFilter(filter); err != nil {
		return nil, err
	}
	if filter.Limit <= 0 {
		filter.Limit = 25
	}
	if filter.Limit > 200 {
		filter.Limit = 200
	}
	if filter.Offset < 0 {
		filter.Offset = 0
	}
	where, args := mysqlTransactionWhere(filter)
	args = append(args, filter.Limit, filter.Offset)
	rows, err := s.db.QueryContext(ctx, `select transactions.id,transactions.account_id,coalesce(transactions.category_id,0),date_format(transactions.booked_on,'%Y-%m-%d'),transactions.description,coalesce(transactions.counterparty,''),coalesce(transactions.counterparty_iban,''),transactions.amount_cents,coalesce(transactions.external_reference,''),coalesce(transactions.import_fingerprint,''),transactions.hidden,(select count(distinct source_id) from entity_relations where source_type='document' and target_type='transaction' and target_id=transactions.id and relation_type='evidence_for') from transactions left join categories on categories.id=transactions.category_id`+where+` order by transactions.booked_on desc,transactions.id desc limit ? offset ?`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanMySQLTransactions(rows)
}

func scanMySQLTransactions(rows *sql.Rows) ([]Transaction, error) {
	items := []Transaction{}
	for rows.Next() {
		var v Transaction
		if err := rows.Scan(&v.ID, &v.AccountID, &v.CategoryID, &v.BookedOn, &v.Description, &v.Counterparty, &v.CounterpartyIBAN, &v.AmountCents, &v.ExternalReference, &v.ImportFingerprint, &v.Hidden, &v.DocumentCount); err != nil {
			return nil, err
		}
		items = append(items, v)
	}
	return items, rows.Err()
}

func (s *MySQL) TransactionYears(ctx context.Context) ([]int, error) {
	rows, err := s.db.QueryContext(ctx, `select distinct year(booked_on) from transactions where hidden=false order by year(booked_on) desc`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []int{}
	for rows.Next() {
		var v int
		if err := rows.Scan(&v); err != nil {
			return nil, err
		}
		items = append(items, v)
	}
	return items, rows.Err()
}

func (s *MySQL) TransactionSummary(ctx context.Context, filter TransactionFilter) (TransactionSummary, error) {
	if err := mysqlValidateTransactionFilter(filter); err != nil {
		return TransactionSummary{}, err
	}
	where, args := mysqlTransactionWhere(filter)
	var v TransactionSummary
	err := s.db.QueryRowContext(ctx, `select count(*),coalesce(sum(transactions.amount_cents),0) from transactions left join categories on categories.id=transactions.category_id`+where, args...).Scan(&v.Count, &v.AmountCents)
	return v, err
}

func (s *MySQL) SetTransactionHidden(ctx context.Context, id int64, hidden bool) error {
	r, err := s.db.ExecContext(ctx, `update transactions set hidden=? where id=?`, hidden, id)
	return mysqlDelete(r, err, "transaction not found")
}

func (s *MySQL) CategorizeTransaction(ctx context.Context, id int64, input CategorizeTransaction) (CategorizeResult, error) {
	if input.CategoryID == 0 {
		return CategorizeResult{}, fmt.Errorf("category is required")
	}
	if !input.ApplyToSameCounterparty {
		r, err := s.db.ExecContext(ctx, `update transactions set category_id=? where id=?`, input.CategoryID, id)
		if err != nil {
			return CategorizeResult{}, err
		}
		n, _ := r.RowsAffected()
		if n == 0 {
			return CategorizeResult{}, fmt.Errorf("transaction not found")
		}
		return CategorizeResult{Updated: int(n)}, nil
	}
	var accountID int64
	var counterparty string
	if err := s.db.QueryRowContext(ctx, `select account_id,coalesce(counterparty,'') from transactions where id=?`, id).Scan(&accountID, &counterparty); err != nil {
		return CategorizeResult{}, fmt.Errorf("transaction not found")
	}
	query := `update transactions set category_id=? where account_id=? and counterparty=? and category_id is null`
	args := []any{input.CategoryID, accountID, counterparty}
	if strings.TrimSpace(counterparty) == "" {
		query = `update transactions set category_id=? where id=?`
		args = []any{input.CategoryID, id}
	}
	r, err := s.db.ExecContext(ctx, query, args...)
	if err != nil {
		return CategorizeResult{}, err
	}
	n, _ := r.RowsAffected()
	return CategorizeResult{Updated: int(n)}, nil
}

func (s *MySQL) CategoryMonthlyTotals(ctx context.Context, filter CategoryReportFilter) ([]MonthlyTotal, error) {
	rows, err := s.db.QueryContext(ctx, `select date_format(transactions.booked_on,'%Y-%m'),sum(transactions.amount_cents) from transactions join categories on categories.id=transactions.category_id where (?=0 or transactions.category_id=?) and (?=0 or categories.main_category_id=?) and transactions.hidden=false group by date_format(transactions.booked_on,'%Y-%m') order by 1`, filter.CategoryID, filter.CategoryID, filter.MainCategoryID, filter.MainCategoryID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []MonthlyTotal{}
	for rows.Next() {
		var v MonthlyTotal
		if err := rows.Scan(&v.Month, &v.AmountCents); err != nil {
			return nil, err
		}
		items = append(items, v)
	}
	return items, rows.Err()
}

func (s *MySQL) MainCategoryPeriodTotals(ctx context.Context, filter PeriodReportFilter) ([]CategoryTotal, error) {
	if _, err := mysqlCanonicalDate(filter.From, true); err != nil {
		return nil, fmt.Errorf("invalid from date")
	}
	if _, err := mysqlCanonicalDate(filter.To, true); err != nil {
		return nil, fmt.Errorf("invalid to date")
	}
	if filter.From != "" && filter.To != "" && filter.From > filter.To {
		return nil, fmt.Errorf("to date cannot be before from date")
	}
	query := `select main_categories.id,main_categories.name,0,'',sum(case when categories.kind='expense' and transactions.amount_cents<0 then abs(transactions.amount_cents) else 0 end) amount_cents from transactions join categories on categories.id=transactions.category_id join main_categories on main_categories.id=categories.main_category_id where transactions.hidden=false and categories.kind='expense' and transactions.amount_cents<0 and (?='' or transactions.booked_on>=?) and (?='' or transactions.booked_on<=?) group by main_categories.id,main_categories.name,main_categories.sort_order order by amount_cents desc,main_categories.sort_order`
	args := []any{filter.From, filter.From, filter.To, filter.To}
	if filter.MainCategoryID != 0 {
		query = `select main_categories.id,main_categories.name,categories.id,categories.name,sum(case when categories.kind='income' and transactions.amount_cents>0 then transactions.amount_cents when categories.kind='expense' and transactions.amount_cents<0 then abs(transactions.amount_cents) else 0 end) amount_cents from transactions join categories on categories.id=transactions.category_id join main_categories on main_categories.id=categories.main_category_id where transactions.hidden=false and main_categories.id=? and ((categories.kind='income' and transactions.amount_cents>0) or (categories.kind='expense' and transactions.amount_cents<0)) and (?='' or transactions.booked_on>=?) and (?='' or transactions.booked_on<=?) group by main_categories.id,main_categories.name,categories.id,categories.name,categories.sort_order order by amount_cents desc,categories.sort_order`
		args = []any{filter.MainCategoryID, filter.From, filter.From, filter.To, filter.To}
	}
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []CategoryTotal{}
	for rows.Next() {
		var v CategoryTotal
		if err := rows.Scan(&v.MainCategoryID, &v.MainCategoryName, &v.CategoryID, &v.CategoryName, &v.AmountCents); err != nil {
			return nil, err
		}
		items = append(items, v)
	}
	return items, rows.Err()
}
