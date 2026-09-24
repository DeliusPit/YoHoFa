package store

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

type CreateManualTransaction struct {
	AccountID    int64  `json:"accountId"`
	CategoryID   int64  `json:"categoryId,omitempty"`
	BookedOn     string `json:"bookedOn"`
	Description  string `json:"description"`
	Counterparty string `json:"counterparty"`
	AmountCents  int64  `json:"amountCents"`
}

type ManualTransactionRepository interface {
	CreateManualTransaction(context.Context, CreateManualTransaction) (Transaction, error)
}

func normalizeManualTransaction(input CreateManualTransaction) (CreateManualTransaction, error) {
	input.BookedOn = strings.TrimSpace(input.BookedOn)
	input.Description = strings.TrimSpace(input.Description)
	input.Counterparty = strings.TrimSpace(input.Counterparty)
	if input.AccountID <= 0 {
		return CreateManualTransaction{}, fmt.Errorf("account is required")
	}
	parsed, err := time.Parse("2006-01-02", input.BookedOn)
	if err != nil || parsed.Format("2006-01-02") != input.BookedOn {
		return CreateManualTransaction{}, fmt.Errorf("invalid booked date")
	}
	if input.Description == "" {
		return CreateManualTransaction{}, fmt.Errorf("description is required")
	}
	if input.AmountCents == 0 {
		return CreateManualTransaction{}, fmt.Errorf("amount must not be zero")
	}
	return input, nil
}

func validateManualDirection(kind string, amount int64) error {
	if (kind == "income" && amount < 0) || (kind == "expense" && amount > 0) {
		return fmt.Errorf("category does not match transaction direction")
	}
	return nil
}

func manualTransactionResult(id int64, input CreateManualTransaction) Transaction {
	return Transaction{ID: id, AccountID: input.AccountID, CategoryID: input.CategoryID, BookedOn: input.BookedOn, Description: input.Description, Counterparty: input.Counterparty, AmountCents: input.AmountCents}
}

func (s *Postgres) CreateManualTransaction(ctx context.Context, input CreateManualTransaction) (Transaction, error) {
	input, err := normalizeManualTransaction(input)
	if err != nil {
		return Transaction{}, err
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Transaction{}, err
	}
	defer tx.Rollback(ctx)
	var accountID int64
	if err := tx.QueryRow(ctx, `select id from accounts where id=$1 for key share`, input.AccountID).Scan(&accountID); err == pgx.ErrNoRows {
		return Transaction{}, fmt.Errorf("account not found")
	} else if err != nil {
		return Transaction{}, err
	}
	var category any
	if input.CategoryID != 0 {
		var kind string
		if err := tx.QueryRow(ctx, `select kind from categories where id=$1 for key share`, input.CategoryID).Scan(&kind); err == pgx.ErrNoRows {
			return Transaction{}, fmt.Errorf("category not found")
		} else if err != nil {
			return Transaction{}, err
		}
		if err := validateManualDirection(kind, input.AmountCents); err != nil {
			return Transaction{}, err
		}
		category = input.CategoryID
	}
	var id int64
	partyID, err := ensurePostgresPartyForTransaction(ctx, tx, input.AccountID, input.Counterparty, "")
	if err != nil {
		return Transaction{}, err
	}
	if err := tx.QueryRow(ctx, `insert into transactions(account_id,category_id,booked_on,description,counterparty,counterparty_party_id,counterparty_name_raw,amount_cents) values($1,$2,$3,$4,$5,$6,$7,$8) returning id`, input.AccountID, category, input.BookedOn, input.Description, input.Counterparty, nullableInt64Value(partyID), input.Counterparty, input.AmountCents).Scan(&id); err != nil {
		return Transaction{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Transaction{}, err
	}
	return manualTransactionResult(id, input), nil
}

func (s *SQLite) CreateManualTransaction(ctx context.Context, input CreateManualTransaction) (Transaction, error) {
	input, err := normalizeManualTransaction(input)
	if err != nil {
		return Transaction{}, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Transaction{}, err
	}
	defer tx.Rollback()
	var accountID int64
	if err := tx.QueryRowContext(ctx, `select id from accounts where id=?`, input.AccountID).Scan(&accountID); err == sql.ErrNoRows {
		return Transaction{}, fmt.Errorf("account not found")
	} else if err != nil {
		return Transaction{}, err
	}
	var category any
	if input.CategoryID != 0 {
		var kind string
		if err := tx.QueryRowContext(ctx, `select kind from categories where id=?`, input.CategoryID).Scan(&kind); err == sql.ErrNoRows {
			return Transaction{}, fmt.Errorf("category not found")
		} else if err != nil {
			return Transaction{}, err
		}
		if err := validateManualDirection(kind, input.AmountCents); err != nil {
			return Transaction{}, err
		}
		category = input.CategoryID
	}
	partyID, err := ensureSQLPartyForTransaction(ctx, tx, input.AccountID, input.Counterparty, "", "sqlite")
	if err != nil {
		return Transaction{}, err
	}
	result, err := tx.ExecContext(ctx, `insert into transactions(account_id,category_id,booked_on,description,counterparty,counterparty_party_id,counterparty_name_raw,amount_cents) values(?,?,?,?,?,?,?,?)`, input.AccountID, category, input.BookedOn, input.Description, input.Counterparty, nullableInt64Value(partyID), input.Counterparty, input.AmountCents)
	if err != nil {
		return Transaction{}, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return Transaction{}, err
	}
	if err := tx.Commit(); err != nil {
		return Transaction{}, err
	}
	return manualTransactionResult(id, input), nil
}

func (s *MySQL) CreateManualTransaction(ctx context.Context, input CreateManualTransaction) (Transaction, error) {
	input, err := normalizeManualTransaction(input)
	if err != nil {
		return Transaction{}, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Transaction{}, err
	}
	defer tx.Rollback()
	var accountID int64
	if err := tx.QueryRowContext(ctx, `select id from accounts where id=? for share`, input.AccountID).Scan(&accountID); err == sql.ErrNoRows {
		return Transaction{}, fmt.Errorf("account not found")
	} else if err != nil {
		return Transaction{}, err
	}
	var category any
	if input.CategoryID != 0 {
		var kind string
		if err := tx.QueryRowContext(ctx, `select kind from categories where id=? for share`, input.CategoryID).Scan(&kind); err == sql.ErrNoRows {
			return Transaction{}, fmt.Errorf("category not found")
		} else if err != nil {
			return Transaction{}, err
		}
		if err := validateManualDirection(kind, input.AmountCents); err != nil {
			return Transaction{}, err
		}
		category = input.CategoryID
	}
	partyID, err := ensureSQLPartyForTransaction(ctx, tx, input.AccountID, input.Counterparty, "", "mysql")
	if err != nil {
		return Transaction{}, err
	}
	result, err := tx.ExecContext(ctx, `insert into transactions(account_id,category_id,booked_on,description,counterparty,counterparty_party_id,counterparty_name_raw,amount_cents) values(?,?,?,?,?,?,?,?)`, input.AccountID, category, input.BookedOn, input.Description, input.Counterparty, nullableInt64Value(partyID), input.Counterparty, input.AmountCents)
	if err != nil {
		return Transaction{}, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return Transaction{}, err
	}
	if err := tx.Commit(); err != nil {
		return Transaction{}, err
	}
	return manualTransactionResult(id, input), nil
}
