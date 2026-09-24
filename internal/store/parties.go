package store

import (
	"context"
	"database/sql"
	"strings"

	"github.com/jackc/pgx/v5"
)

func normalizedIdentifier(value string) string {
	return strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(value), " ", ""))
}

func ensurePostgresPartyForTransaction(ctx context.Context, tx pgx.Tx, accountID int64, name string, iban string) (int64, error) {
	name = strings.TrimSpace(name)
	normalizedIBAN := normalizedIdentifier(iban)
	if name == "" && normalizedIBAN == "" {
		return 0, nil
	}
	var householdID int64
	if err := tx.QueryRow(ctx, `select household_id from accounts where id = $1`, accountID).Scan(&householdID); err != nil {
		return 0, err
	}
	var id int64
	if normalizedIBAN != "" {
		err := tx.QueryRow(ctx, `select party_id from party_identifiers where household_id = $1 and identifier_type = 'iban' and normalized_value = $2 order by id limit 1`, householdID, normalizedIBAN).Scan(&id)
		if err == nil {
			return id, nil
		}
		if err != pgx.ErrNoRows {
			return 0, err
		}
	}
	if name != "" {
		err := tx.QueryRow(ctx, `select id from parties where household_id = $1 and lower(name) = lower($2) order by id limit 1`, householdID, name).Scan(&id)
		if err == nil {
			return ensurePostgresPartyIdentifier(ctx, tx, householdID, id, iban)
		}
		if err != pgx.ErrNoRows {
			return 0, err
		}
	} else {
		name = normalizedIBAN
	}
	if err := tx.QueryRow(ctx, `insert into parties (household_id, party_type, name) values ($1, 'organization', $2) returning id`, householdID, name).Scan(&id); err != nil {
		return 0, err
	}
	return ensurePostgresPartyIdentifier(ctx, tx, householdID, id, iban)
}

func ensurePostgresPartyIdentifier(ctx context.Context, tx pgx.Tx, householdID int64, partyID int64, iban string) (int64, error) {
	normalizedIBAN := normalizedIdentifier(iban)
	if normalizedIBAN == "" {
		return partyID, nil
	}
	_, err := tx.Exec(ctx, `
		insert into party_identifiers (household_id, party_id, identifier_type, identifier_value, normalized_value)
		values ($1, $2, 'iban', $3, $4)
		on conflict (household_id, identifier_type, normalized_value) do nothing
	`, householdID, partyID, strings.TrimSpace(iban), normalizedIBAN)
	return partyID, err
}

func ensureSQLPartyForTransaction(ctx context.Context, tx *sql.Tx, accountID int64, name string, iban string, dialect string) (int64, error) {
	name = strings.TrimSpace(name)
	normalizedIBAN := normalizedIdentifier(iban)
	if name == "" && normalizedIBAN == "" {
		return 0, nil
	}
	var householdID int64
	if err := tx.QueryRowContext(ctx, `select household_id from accounts where id = ?`, accountID).Scan(&householdID); err != nil {
		return 0, err
	}
	var id int64
	if normalizedIBAN != "" {
		err := tx.QueryRowContext(ctx, `select party_id from party_identifiers where household_id = ? and identifier_type = 'iban' and normalized_value = ? order by id limit 1`, householdID, normalizedIBAN).Scan(&id)
		if err == nil {
			return id, nil
		}
		if err != sql.ErrNoRows {
			return 0, err
		}
	}
	if name != "" {
		err := tx.QueryRowContext(ctx, `select id from parties where household_id = ? and lower(name) = lower(?) order by id limit 1`, householdID, name).Scan(&id)
		if err == nil {
			return ensureSQLPartyIdentifier(ctx, tx, householdID, id, iban, dialect)
		}
		if err != sql.ErrNoRows {
			return 0, err
		}
	} else {
		name = normalizedIBAN
	}
	result, err := tx.ExecContext(ctx, `insert into parties (household_id, party_type, name) values (?, 'organization', ?)`, householdID, name)
	if err != nil {
		return 0, err
	}
	id, err = result.LastInsertId()
	if err != nil {
		return 0, err
	}
	return ensureSQLPartyIdentifier(ctx, tx, householdID, id, iban, dialect)
}

func ensureSQLPartyIdentifier(ctx context.Context, tx *sql.Tx, householdID int64, partyID int64, iban string, dialect string) (int64, error) {
	normalizedIBAN := normalizedIdentifier(iban)
	if normalizedIBAN == "" {
		return partyID, nil
	}
	var err error
	if dialect == "mysql" {
		_, err = tx.ExecContext(ctx, `insert into party_identifiers (household_id, party_id, identifier_type, identifier_value, normalized_value) values (?, ?, 'iban', ?, ?) on duplicate key update id = id`, householdID, partyID, strings.TrimSpace(iban), normalizedIBAN)
	} else {
		_, err = tx.ExecContext(ctx, `insert into party_identifiers (household_id, party_id, identifier_type, identifier_value, normalized_value) values (?, ?, 'iban', ?, ?) on conflict(household_id, identifier_type, normalized_value) do nothing`, householdID, partyID, strings.TrimSpace(iban), normalizedIBAN)
	}
	return partyID, err
}

func ensureSQLPartyByName(ctx context.Context, tx *sql.Tx, name string) (int64, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return 0, nil
	}
	var householdID int64
	if err := tx.QueryRowContext(ctx, `select id from households order by id limit 1`).Scan(&householdID); err != nil {
		return 0, err
	}
	var id int64
	err := tx.QueryRowContext(ctx, `select id from parties where household_id = ? and lower(name) = lower(?) order by id limit 1`, householdID, name).Scan(&id)
	if err == nil {
		return id, nil
	}
	if err != sql.ErrNoRows {
		return 0, err
	}
	result, err := tx.ExecContext(ctx, `insert into parties (household_id, party_type, name) values (?, 'organization', ?)`, householdID, name)
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}
