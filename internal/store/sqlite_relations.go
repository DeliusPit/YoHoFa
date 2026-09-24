package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

func (s *SQLite) CreateEntityRelation(ctx context.Context, input SaveEntityRelation) (EntityRelation, error) {
	input, validFrom, validTo, err := normalizeEntityRelation(input)
	if err != nil {
		return EntityRelation{}, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return EntityRelation{}, err
	}
	defer tx.Rollback()
	if err := sqliteValidateEntity(ctx, tx, input.SourceType, input.SourceID); err != nil {
		return EntityRelation{}, err
	}
	if err := sqliteValidateEntity(ctx, tx, input.TargetType, input.TargetID); err != nil {
		return EntityRelation{}, err
	}
	validFrom = sqliteOptionalDate(input.ValidFrom)
	validTo = sqliteOptionalDate(input.ValidTo)
	result, err := tx.ExecContext(ctx, `
		insert into entity_relations (source_type, source_id, target_type, target_id, relation_type, valid_from, valid_to, notes)
		values (?, ?, ?, ?, ?, ?, ?, ?)
		on conflict do nothing
	`, input.SourceType, input.SourceID, input.TargetType, input.TargetID, input.RelationType, validFrom, validTo, input.Notes)
	if err != nil {
		return EntityRelation{}, err
	}
	inserted, _ := result.RowsAffected()
	var item EntityRelation
	err = scanSQLiteRelation(tx.QueryRowContext(ctx, sqliteRelationSelect+`
		where source_type = ? and source_id = ? and target_type = ? and target_id = ? and relation_type = ?
		and coalesce(valid_from, '') = coalesce(?, '') and coalesce(valid_to, '') = coalesce(?, '')
	`, input.SourceType, input.SourceID, input.TargetType, input.TargetID, input.RelationType, validFrom, validTo), &item)
	if err != nil {
		return EntityRelation{}, err
	}
	if inserted != 0 {
		if err := appendSQLiteRelationDomainEvents(ctx, tx, item, true); err != nil {
			return EntityRelation{}, err
		}
	}
	if err := tx.Commit(); err != nil {
		return EntityRelation{}, err
	}
	if err := s.addSQLiteEntityLabels(ctx, &item); err != nil {
		return EntityRelation{}, err
	}
	return item, nil
}

func (s *SQLite) DeleteEntityRelation(ctx context.Context, id int64, objectType string, objectID int64) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var item EntityRelation
	err = scanSQLiteRelation(tx.QueryRowContext(ctx, sqliteRelationSelect+`
		where id = ? and ((source_type = ? and source_id = ?) or (target_type = ? and target_id = ?))
	`, id, objectType, objectID, objectType, objectID), &item)
	if err == sql.ErrNoRows {
		return fmt.Errorf("entity relation not found")
	}
	if err != nil {
		return err
	}
	if err := sqliteValidateEntity(ctx, tx, item.SourceType, item.SourceID); err != nil {
		return err
	}
	if err := sqliteValidateEntity(ctx, tx, item.TargetType, item.TargetID); err != nil {
		return err
	}
	result, err := tx.ExecContext(ctx, `delete from entity_relations where id = ?`, id)
	if err != nil {
		return err
	}
	if affected, _ := result.RowsAffected(); affected == 0 {
		return fmt.Errorf("entity relation not found")
	}
	if err := appendSQLiteRelationDomainEvents(ctx, tx, item, false); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *SQLite) LinkAssetTransaction(ctx context.Context, assetID, transactionID int64) error {
	return s.sqliteLinkObjectTransaction(ctx, ObjectTypeAsset, assetID, transactionID)
}

func (s *SQLite) UnlinkAssetTransaction(ctx context.Context, assetID, transactionID int64) error {
	return s.sqliteUnlinkObjectTransaction(ctx, ObjectTypeAsset, assetID, transactionID)
}

func (s *SQLite) AssetTransactions(ctx context.Context, assetID int64) ([]Transaction, error) {
	return s.sqliteObjectTransactions(ctx, ObjectTypeAsset, assetID)
}

func (s *SQLite) LinkLiabilityTransaction(ctx context.Context, liabilityID, transactionID int64) error {
	return s.sqliteLinkObjectTransaction(ctx, ObjectTypeLiability, liabilityID, transactionID)
}

func (s *SQLite) UnlinkLiabilityTransaction(ctx context.Context, liabilityID, transactionID int64) error {
	return s.sqliteUnlinkObjectTransaction(ctx, ObjectTypeLiability, liabilityID, transactionID)
}

func (s *SQLite) LiabilityTransactions(ctx context.Context, liabilityID int64) ([]Transaction, error) {
	return s.sqliteObjectTransactions(ctx, ObjectTypeLiability, liabilityID)
}

func (s *SQLite) LinkCaseTransaction(ctx context.Context, caseID, transactionID int64) error {
	return s.sqliteLinkObjectTransaction(ctx, ObjectTypeCase, caseID, transactionID)
}

func (s *SQLite) UnlinkCaseTransaction(ctx context.Context, caseID, transactionID int64) error {
	return s.sqliteUnlinkObjectTransaction(ctx, ObjectTypeCase, caseID, transactionID)
}

func (s *SQLite) CaseTransactions(ctx context.Context, caseID int64) ([]Transaction, error) {
	return s.sqliteObjectTransactions(ctx, ObjectTypeCase, caseID)
}

func (s *SQLite) RelationsForObject(ctx context.Context, objectType string, objectID int64) ([]EntityRelation, error) {
	rows, err := s.db.QueryContext(ctx, sqliteRelationSelect+`
		where (source_type = ? and source_id = ?) or (target_type = ? and target_id = ?)
		order by created_at, id
	`, objectType, objectID, objectType, objectID)
	if err != nil {
		return nil, err
	}
	items := []EntityRelation{}
	for rows.Next() {
		var item EntityRelation
		if err := scanSQLiteRelation(rows, &item); err != nil {
			rows.Close()
			return nil, err
		}
		items = append(items, item)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	for i := range items {
		if err := s.addSQLiteEntityLabels(ctx, &items[i]); err != nil {
			return nil, err
		}
	}
	return items, nil
}

func (s *SQLite) sqliteLinkObjectTransaction(ctx context.Context, objectType string, objectID, transactionID int64) error {
	_, err := s.CreateEntityRelation(ctx, SaveEntityRelation{
		SourceType: objectType, SourceID: objectID, TargetType: ObjectTypeTransaction, TargetID: transactionID,
		RelationType: RelationTypeRelatesTo,
	})
	return err
}

func (s *SQLite) sqliteUnlinkObjectTransaction(ctx context.Context, objectType string, objectID, transactionID int64) error {
	if !validDomainEventOwner(objectType) {
		return fmt.Errorf("unsupported object type")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := sqliteValidateEntity(ctx, tx, objectType, objectID); err != nil {
		return err
	}
	if err := sqliteValidateEntity(ctx, tx, ObjectTypeTransaction, transactionID); err != nil {
		return err
	}
	result, err := tx.ExecContext(ctx, `
		delete from entity_relations where source_type = ? and source_id = ? and target_type = 'transaction' and target_id = ? and relation_type = 'relates_to'
	`, objectType, objectID, transactionID)
	if err != nil {
		return err
	}
	if affected, _ := result.RowsAffected(); affected == 0 {
		return fmt.Errorf("%s transaction link not found", objectType)
	}
	if err := appendSQLiteDomainEvent(ctx, tx, AppendDomainEvent{
		EntityType: objectType, EntityID: objectID, EventType: "transaction_unlinked", CurrentDate: true,
		Description: "Transactie ontkoppeld", RelatedEntityType: ObjectTypeTransaction, RelatedEntityID: transactionID,
	}); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *SQLite) sqliteObjectTransactions(ctx context.Context, objectType string, objectID int64) ([]Transaction, error) {
	rows, err := s.db.QueryContext(ctx, `
		select transactions.id, transactions.account_id, coalesce(transactions.category_id, 0), transactions.booked_on, transactions.description,
			coalesce(parties.name, transactions.counterparty_name_raw, transactions.counterparty, ''), coalesce(transactions.counterparty_iban_raw, transactions.counterparty_iban, ''), transactions.amount_cents,
			coalesce(transactions.external_reference, ''), coalesce(transactions.import_fingerprint, ''), transactions.hidden,
			(select count(distinct source_id) from entity_relations where source_type = 'document' and target_type = 'transaction' and target_id = transactions.id and relation_type = 'evidence_for')
		from transactions
		left join parties on parties.id = transactions.counterparty_party_id
		where exists (
			select 1 from entity_relations where target_type = 'transaction' and target_id = transactions.id
			and source_type = ? and source_id = ? and relation_type = 'relates_to'
		)
		order by transactions.booked_on desc, transactions.id desc
	`, objectType, objectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []Transaction{}
	for rows.Next() {
		var item Transaction
		if err := rows.Scan(&item.ID, &item.AccountID, &item.CategoryID, &item.BookedOn, &item.Description, &item.Counterparty,
			&item.CounterpartyIBAN, &item.AmountCents, &item.ExternalReference, &item.ImportFingerprint, &item.Hidden, &item.DocumentCount); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func appendSQLiteRelationDomainEvents(ctx context.Context, tx *sql.Tx, relation EntityRelation, linked bool) error {
	eventDate := time.Time{}
	currentDate := true
	if linked {
		var date string
		switch {
		case relation.RelationType == RelationTypeEvidenceFor:
			if err := tx.QueryRowContext(ctx, `select coalesce(document_date, strftime('%Y-%m-%d', 'now')) from documents where id = ?`, relation.SourceID).Scan(&date); err != nil {
				return err
			}
		case relation.RelationType == RelationTypeRelatesTo && relation.TargetType == ObjectTypeTransaction:
			if err := tx.QueryRowContext(ctx, `select booked_on from transactions where id = ?`, relation.TargetID).Scan(&date); err != nil {
				return err
			}
		case relation.ValidFrom != "":
			date = relation.ValidFrom
		}
		if date != "" {
			var err error
			eventDate, err = time.Parse("2006-01-02", date)
			if err != nil {
				return err
			}
			currentDate = false
		}
	}
	if relation.RelationType == RelationTypeEvidenceFor && validDomainEventOwner(relation.TargetType) {
		eventType, description := "document_linked", "Document gekoppeld"
		if !linked {
			eventType, description = "document_unlinked", "Document ontkoppeld"
		}
		return appendSQLiteDomainEvent(ctx, tx, AppendDomainEvent{EntityType: relation.TargetType, EntityID: relation.TargetID, EventType: eventType,
			EventDate: eventDate, CurrentDate: currentDate, Description: description, RelatedEntityType: ObjectTypeDocument, RelatedEntityID: relation.SourceID})
	}
	if relation.RelationType == RelationTypeRelatesTo && relation.TargetType == ObjectTypeTransaction && validDomainEventOwner(relation.SourceType) {
		eventType, description := "transaction_linked", "Transactie gekoppeld"
		if !linked {
			eventType, description = "transaction_unlinked", "Transactie ontkoppeld"
		}
		return appendSQLiteDomainEvent(ctx, tx, AppendDomainEvent{EntityType: relation.SourceType, EntityID: relation.SourceID, EventType: eventType,
			EventDate: eventDate, CurrentDate: currentDate, Description: description, RelatedEntityType: ObjectTypeTransaction, RelatedEntityID: relation.TargetID})
	}
	objectRelation := relation.RelationType == RelationTypeFinancedBy ||
		(relation.RelationType == RelationTypeRelatesTo && relation.SourceType == ObjectTypeCase &&
			(relation.TargetType == ObjectTypeAsset || relation.TargetType == ObjectTypeLiability))
	if !objectRelation {
		return nil
	}
	eventType, description := "relation_linked", "Gerelateerd object gekoppeld"
	if !linked {
		eventType, description = "relation_unlinked", "Gerelateerd object ontkoppeld"
	}
	if err := appendSQLiteDomainEvent(ctx, tx, AppendDomainEvent{EntityType: relation.SourceType, EntityID: relation.SourceID, EventType: eventType,
		EventDate: eventDate, CurrentDate: currentDate, Description: description, RelatedEntityType: relation.TargetType, RelatedEntityID: relation.TargetID}); err != nil {
		return err
	}
	return appendSQLiteDomainEvent(ctx, tx, AppendDomainEvent{EntityType: relation.TargetType, EntityID: relation.TargetID, EventType: eventType,
		EventDate: eventDate, CurrentDate: currentDate, Description: description, RelatedEntityType: relation.SourceType, RelatedEntityID: relation.SourceID})
}

func sqliteValidateEntity(ctx context.Context, tx *sql.Tx, entityType string, entityID int64) error {
	var table string
	switch entityType {
	case ObjectTypeAsset:
		table = "assets"
	case ObjectTypeLiability:
		table = "liabilities"
	case ObjectTypeTransaction:
		table = "transactions"
	case ObjectTypeDocument:
		table = "documents"
	case ObjectTypeCase:
		table = "cases"
	case ObjectTypeReservation:
		table = "reservations"
	default:
		return fmt.Errorf("unsupported entity type")
	}
	var id int64
	if err := tx.QueryRowContext(ctx, "select id from "+table+" where id = ?", entityID).Scan(&id); err == sql.ErrNoRows {
		return fmt.Errorf("%s not found", entityType)
	} else {
		return err
	}
}

func (s *SQLite) addSQLiteEntityLabels(ctx context.Context, relation *EntityRelation) error {
	var err error
	relation.SourceLabel, err = s.sqliteEntityLabel(ctx, relation.SourceType, relation.SourceID)
	if err != nil {
		return err
	}
	relation.TargetLabel, err = s.sqliteEntityLabel(ctx, relation.TargetType, relation.TargetID)
	return err
}

const sqliteRelationSelect = `
	select id, source_type, source_id, target_type, target_id, relation_type,
		coalesce(valid_from, ''), coalesce(valid_to, ''), coalesce(notes, ''), created_at
	from entity_relations`

func scanSQLiteRelation(scanner interface{ Scan(...any) error }, item *EntityRelation) error {
	return scanner.Scan(&item.ID, &item.SourceType, &item.SourceID, &item.TargetType, &item.TargetID, &item.RelationType,
		&item.ValidFrom, &item.ValidTo, &item.Notes, &item.CreatedAt)
}

func sqliteOptionalDate(value string) any {
	if value == "" {
		return nil
	}
	return value
}
