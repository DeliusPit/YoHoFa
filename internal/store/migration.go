package store

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/jackc/pgx/v5/pgconn"
	_ "modernc.org/sqlite"
)

type MigrationResult struct {
	Households     int
	Accounts       int
	MainCategories int
	Categories     int
	Transactions   int
}

func MigrateSQLiteToPostgres(ctx context.Context, sqlitePath string, databaseURL string) (MigrationResult, error) {
	sqliteDB, err := sql.Open("sqlite", sqlitePath)
	if err != nil {
		return MigrationResult{}, err
	}
	defer sqliteDB.Close()
	if err := sqliteDB.PingContext(ctx); err != nil {
		return MigrationResult{}, err
	}

	postgres, err := OpenPostgres(ctx, databaseURL)
	if err != nil {
		return MigrationResult{}, err
	}
	defer postgres.Close()

	tx, err := postgres.pool.Begin(ctx)
	if err != nil {
		return MigrationResult{}, err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `
		lock table transactions, categories, accounts, main_categories, households, assets, liabilities, cases,
			document_links, object_relations, entity_relations, domain_events in access exclusive mode
	`); err != nil {
		return MigrationResult{}, err
	}

	var assetCount, liabilityCount, caseCount, transactionDocumentLinkCount, assetDocumentLinkCount, liabilityDocumentLinkCount, caseDocumentLinkCount int
	var transactionRelationCount, assetRelationCount, liabilityRelationCount, caseRelationCount, entityRelationCount, domainEventCount int
	if err := tx.QueryRow(ctx, `select count(*) from assets`).Scan(&assetCount); err != nil {
		return MigrationResult{}, err
	}
	if err := tx.QueryRow(ctx, `select count(*) from liabilities`).Scan(&liabilityCount); err != nil {
		return MigrationResult{}, err
	}
	if err := tx.QueryRow(ctx, `select count(*) from cases`).Scan(&caseCount); err != nil {
		return MigrationResult{}, err
	}
	if err := tx.QueryRow(ctx, `select count(*) from document_links where object_type = $1`, ObjectTypeTransaction).Scan(&transactionDocumentLinkCount); err != nil {
		return MigrationResult{}, err
	}
	if err := tx.QueryRow(ctx, `select count(*) from document_links where object_type = $1`, ObjectTypeAsset).Scan(&assetDocumentLinkCount); err != nil {
		return MigrationResult{}, err
	}
	if err := tx.QueryRow(ctx, `select count(*) from document_links where object_type = $1`, ObjectTypeLiability).Scan(&liabilityDocumentLinkCount); err != nil {
		return MigrationResult{}, err
	}
	if err := tx.QueryRow(ctx, `select count(*) from document_links where object_type = $1`, ObjectTypeCase).Scan(&caseDocumentLinkCount); err != nil {
		return MigrationResult{}, err
	}
	if err := tx.QueryRow(ctx, `
		select count(*) from object_relations
		where object_type = $1 or related_object_type = $1
	`, ObjectTypeTransaction).Scan(&transactionRelationCount); err != nil {
		return MigrationResult{}, err
	}
	if err := tx.QueryRow(ctx, `
		select count(*) from object_relations
		where object_type = $1 or related_object_type = $1
	`, ObjectTypeAsset).Scan(&assetRelationCount); err != nil {
		return MigrationResult{}, err
	}
	if err := tx.QueryRow(ctx, `
		select count(*) from object_relations
		where object_type = $1 or related_object_type = $1
	`, ObjectTypeLiability).Scan(&liabilityRelationCount); err != nil {
		return MigrationResult{}, err
	}
	if err := tx.QueryRow(ctx, `
		select count(*) from object_relations
		where object_type = $1 or related_object_type = $1
	`, ObjectTypeCase).Scan(&caseRelationCount); err != nil {
		return MigrationResult{}, err
	}
	if err := tx.QueryRow(ctx, `
		select count(*) from entity_relations
		where source_type in ($1, $2, $3, $4) or target_type in ($1, $2, $3, $4)
	`, ObjectTypeAsset, ObjectTypeTransaction, ObjectTypeLiability, ObjectTypeCase).Scan(&entityRelationCount); err != nil {
		return MigrationResult{}, err
	}
	if err := tx.QueryRow(ctx, `select count(*) from domain_events`).Scan(&domainEventCount); err != nil {
		return MigrationResult{}, err
	}
	if assetCount > 0 || liabilityCount > 0 || caseCount > 0 || transactionDocumentLinkCount > 0 || assetDocumentLinkCount > 0 || liabilityDocumentLinkCount > 0 || caseDocumentLinkCount > 0 || transactionRelationCount > 0 || assetRelationCount > 0 || liabilityRelationCount > 0 || caseRelationCount > 0 || entityRelationCount > 0 || domainEventCount > 0 {
		return MigrationResult{}, fmt.Errorf("migration refused: target contains assets, liabilities, cases, object links, or domain events")
	}
	if _, err := tx.Exec(ctx, `truncate transactions, categories, accounts, main_categories, households restart identity cascade`); err != nil {
		return MigrationResult{}, err
	}

	var result MigrationResult
	if result.Households, err = migrateHouseholds(ctx, sqliteDB, tx); err != nil {
		return MigrationResult{}, err
	}
	if result.MainCategories, err = migrateMainCategories(ctx, sqliteDB, tx); err != nil {
		return MigrationResult{}, err
	}
	if result.Accounts, err = migrateAccounts(ctx, sqliteDB, tx); err != nil {
		return MigrationResult{}, err
	}
	if result.Categories, err = migrateCategories(ctx, sqliteDB, tx); err != nil {
		return MigrationResult{}, err
	}
	if result.Transactions, err = migrateTransactions(ctx, sqliteDB, tx); err != nil {
		return MigrationResult{}, err
	}

	for _, table := range []string{"households", "accounts", "main_categories", "categories", "transactions"} {
		if _, err := tx.Exec(ctx, fmt.Sprintf(`select setval(pg_get_serial_sequence('%s', 'id'), coalesce((select max(id) from %s), 1), true)`, table, table)); err != nil {
			return MigrationResult{}, err
		}
	}

	return result, tx.Commit(ctx)
}

type pgTx interface {
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
}

func migrateHouseholds(ctx context.Context, sqliteDB *sql.DB, tx pgTx) (int, error) {
	rows, err := sqliteDB.QueryContext(ctx, `select id, name, created_at from households order by id`)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	count := 0
	for rows.Next() {
		var id int64
		var name string
		var createdAt string
		if err := rows.Scan(&id, &name, &createdAt); err != nil {
			return 0, err
		}
		if _, err := tx.Exec(ctx, `insert into households (id, name, created_at) values ($1, $2, $3)`, id, name, createdAt); err != nil {
			return 0, err
		}
		count++
	}
	return count, rows.Err()
}

func migrateMainCategories(ctx context.Context, sqliteDB *sql.DB, tx pgTx) (int, error) {
	rows, err := sqliteDB.QueryContext(ctx, `select id, name, sort_order, created_at from main_categories order by id`)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	count := 0
	for rows.Next() {
		var id int64
		var name, createdAt string
		var sortOrder int
		if err := rows.Scan(&id, &name, &sortOrder, &createdAt); err != nil {
			return 0, err
		}
		if _, err := tx.Exec(ctx, `insert into main_categories (id, name, sort_order, created_at) values ($1, $2, $3, $4)`, id, name, sortOrder, createdAt); err != nil {
			return 0, err
		}
		count++
	}
	return count, rows.Err()
}

func migrateAccounts(ctx context.Context, sqliteDB *sql.DB, tx pgTx) (int, error) {
	rows, err := sqliteDB.QueryContext(ctx, `select id, household_id, name, iban, account_type, owner_name, bank_profile, currency, opening_balance_cents, sort_order, created_at from accounts order by id`)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	count := 0
	for rows.Next() {
		var id, householdID, openingBalanceCents int64
		var name, accountType, bankProfile, currency, createdAt string
		var iban, ownerName sql.NullString
		var sortOrder int
		if err := rows.Scan(&id, &householdID, &name, &iban, &accountType, &ownerName, &bankProfile, &currency, &openingBalanceCents, &sortOrder, &createdAt); err != nil {
			return 0, err
		}
		if _, err := tx.Exec(ctx, `
			insert into accounts (id, household_id, name, iban, account_type, owner_name, bank_profile, currency, opening_balance_cents, sort_order, created_at)
			values ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		`, id, householdID, name, nullableString(iban), accountType, nullableString(ownerName), bankProfile, currency, openingBalanceCents, sortOrder, createdAt); err != nil {
			return 0, err
		}
		count++
	}
	return count, rows.Err()
}

func migrateCategories(ctx context.Context, sqliteDB *sql.DB, tx pgTx) (int, error) {
	rows, err := sqliteDB.QueryContext(ctx, `select id, main_category_id, name, kind, purpose, sort_order, created_at from categories order by id`)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	count := 0
	for rows.Next() {
		var id int64
		var mainCategoryID sql.NullInt64
		var name, kind, purpose, createdAt string
		var sortOrder int
		if err := rows.Scan(&id, &mainCategoryID, &name, &kind, &purpose, &sortOrder, &createdAt); err != nil {
			return 0, err
		}
		if _, err := tx.Exec(ctx, `
			insert into categories (id, household_id, main_category_id, name, kind, purpose, sort_order, created_at)
			values ($1, 1, $2, $3, $4, $5, $6, $7)
		`, id, nullableInt64(mainCategoryID), name, kind, purpose, sortOrder, createdAt); err != nil {
			return 0, err
		}
		count++
	}
	return count, rows.Err()
}

func migrateTransactions(ctx context.Context, sqliteDB *sql.DB, tx pgTx) (int, error) {
	rows, err := sqliteDB.QueryContext(ctx, `select id, account_id, category_id, booked_on, description, counterparty, counterparty_iban, amount_cents, external_reference, import_fingerprint, hidden, created_at from transactions order by id`)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	count := 0
	for rows.Next() {
		var id, accountID, amountCents int64
		var categoryID sql.NullInt64
		var bookedOn, description, createdAt string
		var counterparty, counterpartyIBAN, externalReference, importFingerprint sql.NullString
		var hidden bool
		if err := rows.Scan(&id, &accountID, &categoryID, &bookedOn, &description, &counterparty, &counterpartyIBAN, &amountCents, &externalReference, &importFingerprint, &hidden, &createdAt); err != nil {
			return 0, err
		}
		if _, err := tx.Exec(ctx, `
			insert into transactions (id, account_id, category_id, booked_on, description, counterparty, counterparty_iban, amount_cents, external_reference, import_fingerprint, hidden, created_at)
			values ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		`, id, accountID, nullableInt64(categoryID), bookedOn, description, nullableString(counterparty), nullableString(counterpartyIBAN), amountCents, nullableString(externalReference), nullableString(importFingerprint), hidden, createdAt); err != nil {
			return 0, err
		}
		count++
	}
	return count, rows.Err()
}

func nullableString(value sql.NullString) any {
	if !value.Valid {
		return nil
	}
	return value.String
}

func nullableInt64(value sql.NullInt64) any {
	if !value.Valid {
		return nil
	}
	return value.Int64
}
