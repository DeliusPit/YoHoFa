package store

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"fmt"

	"hohoho/internal/bankconfig"
)

type BankImportProfile struct {
	ID            int64  `json:"id"`
	Name          string `json:"name"`
	SourceFormat  string `json:"sourceFormat"`
	Configuration string `json:"configuration"`
}

type CoreBankImportResult struct {
	ProfileID int64 `json:"profileId"`
	Imported  int   `json:"imported"`
	Skipped   int   `json:"skipped"`
	Total     int   `json:"total"`
}

type CoreBankImportRow struct {
	Parsed bankconfig.ParsedRow
}

func (s *Core) UpsertBankImportProfile(ctx context.Context, name, sourceFormat, configuration string) (BankImportProfile, error) {
	if name == "" || sourceFormat == "" || configuration == "" {
		return BankImportProfile{}, ErrCoreInvalid
	}
	var id int64
	err := s.db.QueryRowContext(ctx, "SELECT id FROM bank_import_profiles WHERE name = ?", name).Scan(&id)
	if err == nil {
		_, err = s.db.ExecContext(ctx, "UPDATE bank_import_profiles SET source_format = ?, configuration = ? WHERE id = ?", sourceFormat, configuration, id)
	} else if err == sql.ErrNoRows {
		result, insertErr := s.db.ExecContext(ctx, "INSERT INTO bank_import_profiles (name, source_format, configuration) VALUES (?, ?, ?)", name, sourceFormat, configuration)
		err = insertErr
		if err == nil {
			id, err = result.LastInsertId()
		}
	}
	if err != nil {
		return BankImportProfile{}, coreStoreError(err)
	}
	return s.BankImportProfile(ctx, id)
}

func (s *Core) BankImportProfile(ctx context.Context, id int64) (BankImportProfile, error) {
	var item BankImportProfile
	err := s.db.QueryRowContext(ctx, "SELECT id, name, source_format, configuration FROM bank_import_profiles WHERE id = ?", id).Scan(&item.ID, &item.Name, &item.SourceFormat, &item.Configuration)
	return item, coreStoreError(err)
}

func (s *Core) DeleteBankImportProfileByName(ctx context.Context, name string) error {
	result, err := s.db.ExecContext(ctx, "DELETE FROM bank_import_profiles WHERE name = ? AND id NOT IN (SELECT profile_id FROM bank_import_rows)", name)
	return coreMutationError(result, err)
}

func (s *Core) ImportBankTransactions(ctx context.Context, profile BankImportProfile, rows []CoreBankImportRow) (CoreBankImportResult, error) {
	result := CoreBankImportResult{ProfileID: profile.ID, Total: len(rows)}
	if profile.ID <= 0 || len(rows) == 0 {
		return result, ErrCoreInvalid
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return result, err
	}
	defer tx.Rollback()
	for _, row := range rows {
		amount := row.Parsed.Transaction.AmountCents
		input := SaveCoreTransaction{TransactionDate: row.Parsed.Transaction.TransactionDate, AmountCents: &amount, Description: row.Parsed.Transaction.Description, Counterparty: row.Parsed.Transaction.Counterparty}
		input, err := normalizeCoreTransaction(input)
		if err != nil {
			return result, err
		}
		raw, err := json.Marshal(row.Parsed.Raw)
		if err != nil {
			return result, err
		}
		fingerprint := bankImportFingerprint(input.TransactionDate, amount, input.Description, input.Counterparty)
		stored, err := tx.ExecContext(ctx, "INSERT INTO bank_import_rows (profile_id, raw_data, fingerprint) VALUES (?, ?, ?) ON CONFLICT(profile_id, fingerprint) DO NOTHING", profile.ID, string(raw), fingerprint)
		if err != nil {
			return result, coreStoreError(err)
		}
		affected, err := stored.RowsAffected()
		if err != nil {
			return result, err
		}
		if affected == 0 {
			result.Skipped++
			continue
		}
		inserted, err := tx.ExecContext(ctx, "INSERT INTO transactions (transaction_date, amount_cents, description, counterparty, status) VALUES (?, ?, ?, ?, 'I')", input.TransactionDate, amount, input.Description, input.Counterparty)
		if err != nil {
			return result, coreStoreError(err)
		}
		transactionID, err := inserted.LastInsertId()
		if err != nil {
			return result, err
		}
		if _, err := tx.ExecContext(ctx, "UPDATE bank_import_rows SET transaction_id = ? WHERE profile_id = ? AND fingerprint = ?", transactionID, profile.ID, fingerprint); err != nil {
			return result, coreStoreError(err)
		}
		result.Imported++
	}
	return result, tx.Commit()
}

func bankImportFingerprint(date string, amount int64, description, counterparty string) string {
	data := fmt.Sprintf("%s\x00%d\x00%s\x00%s", date, amount, description, counterparty)
	sum := sha256.Sum256([]byte(data))
	return fmt.Sprintf("%x", sum[:])
}
