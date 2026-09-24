package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

const mysqlRelationColumns = `id,source_type,source_id,target_type,target_id,relation_type,coalesce(date_format(valid_from,'%Y-%m-%d'),''),coalesce(date_format(valid_to,'%Y-%m-%d'),''),coalesce(notes,''),date_format(created_at,'%Y-%m-%d %H:%i:%s.%f+00')`

func (s *MySQL) CreateEntityRelation(ctx context.Context, input SaveEntityRelation) (EntityRelation, error) {
	input, from, to, err := normalizeEntityRelation(input)
	if err != nil {
		return EntityRelation{}, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return EntityRelation{}, err
	}
	defer tx.Rollback()
	firstType, firstID, secondType, secondID := orderedEntities(input.SourceType, input.SourceID, input.TargetType, input.TargetID)
	if err = mysqlValidateAndLockEntity(ctx, tx, firstType, firstID); err != nil {
		return EntityRelation{}, err
	}
	if err = mysqlValidateAndLockEntity(ctx, tx, secondType, secondID); err != nil {
		return EntityRelation{}, err
	}
	r, err := tx.ExecContext(ctx, `insert into entity_relations(source_type,source_id,target_type,target_id,relation_type,valid_from,valid_to,notes) values (?,?,?,?,?,?,?,?)`, input.SourceType, input.SourceID, input.TargetType, input.TargetID, input.RelationType, from, to, input.Notes)
	inserted := err == nil
	var id int64
	if err == nil {
		id, err = r.LastInsertId()
	} else if mysqlDuplicate(err) {
		err = tx.QueryRowContext(ctx, `select id from entity_relations where source_type=? and source_id=? and target_type=? and target_id=? and relation_type=? and valid_from <=> ? and valid_to <=> ? for share`, input.SourceType, input.SourceID, input.TargetType, input.TargetID, input.RelationType, from, to).Scan(&id)
	}
	if err != nil {
		return EntityRelation{}, err
	}
	var v EntityRelation
	if err = scanMySQLRelation(tx.QueryRowContext(ctx, `select `+mysqlRelationColumns+` from entity_relations where id=?`, id), &v); err != nil {
		return EntityRelation{}, err
	}
	if inserted {
		if err = s.appendRelationDomainEvents(ctx, tx, v, true); err != nil {
			return EntityRelation{}, err
		}
	}
	if err = tx.Commit(); err != nil {
		return EntityRelation{}, err
	}
	if err = s.addEntityLabels(ctx, &v); err != nil {
		return EntityRelation{}, err
	}
	return v, nil
}

func orderedEntities(aType string, aID int64, bType string, bID int64) (string, int64, string, int64) {
	if bType < aType || (bType == aType && bID < aID) {
		return bType, bID, aType, aID
	}
	return aType, aID, bType, bID
}
func scanMySQLRelation(scanner interface{ Scan(...any) error }, v *EntityRelation) error {
	return scanner.Scan(&v.ID, &v.SourceType, &v.SourceID, &v.TargetType, &v.TargetID, &v.RelationType, &v.ValidFrom, &v.ValidTo, &v.Notes, &v.CreatedAt)
}

func (s *MySQL) DeleteEntityRelation(ctx context.Context, id int64, objectType string, objectID int64) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var v EntityRelation
	err = scanMySQLRelation(tx.QueryRowContext(ctx, `select `+mysqlRelationColumns+` from entity_relations where id=? and ((source_type=? and source_id=?) or (target_type=? and target_id=?))`, id, objectType, objectID, objectType, objectID), &v)
	if err == sql.ErrNoRows {
		return fmt.Errorf("entity relation not found")
	} else if err != nil {
		return err
	}
	firstType, firstID, secondType, secondID := orderedEntities(v.SourceType, v.SourceID, v.TargetType, v.TargetID)
	if err = mysqlValidateAndLockEntity(ctx, tx, firstType, firstID); err != nil {
		return err
	}
	if err = mysqlValidateAndLockEntity(ctx, tx, secondType, secondID); err != nil {
		return err
	}
	r, err := tx.ExecContext(ctx, `delete from entity_relations where id=?`, id)
	if err != nil {
		return err
	}
	n, _ := r.RowsAffected()
	if n == 0 {
		return fmt.Errorf("entity relation not found")
	}
	if err = s.appendRelationDomainEvents(ctx, tx, v, false); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *MySQL) LinkAssetTransaction(ctx context.Context, a, t int64) error {
	return s.linkObjectTransaction(ctx, ObjectTypeAsset, a, t)
}
func (s *MySQL) UnlinkAssetTransaction(ctx context.Context, a, t int64) error {
	return s.unlinkObjectTransaction(ctx, ObjectTypeAsset, a, t)
}
func (s *MySQL) AssetTransactions(ctx context.Context, id int64) ([]Transaction, error) {
	return s.objectTransactions(ctx, ObjectTypeAsset, id)
}
func (s *MySQL) LinkLiabilityTransaction(ctx context.Context, a, t int64) error {
	return s.linkObjectTransaction(ctx, ObjectTypeLiability, a, t)
}
func (s *MySQL) UnlinkLiabilityTransaction(ctx context.Context, a, t int64) error {
	return s.unlinkObjectTransaction(ctx, ObjectTypeLiability, a, t)
}
func (s *MySQL) LiabilityTransactions(ctx context.Context, id int64) ([]Transaction, error) {
	return s.objectTransactions(ctx, ObjectTypeLiability, id)
}
func (s *MySQL) LinkCaseTransaction(ctx context.Context, a, t int64) error {
	return s.linkObjectTransaction(ctx, ObjectTypeCase, a, t)
}
func (s *MySQL) UnlinkCaseTransaction(ctx context.Context, a, t int64) error {
	return s.unlinkObjectTransaction(ctx, ObjectTypeCase, a, t)
}
func (s *MySQL) CaseTransactions(ctx context.Context, id int64) ([]Transaction, error) {
	return s.objectTransactions(ctx, ObjectTypeCase, id)
}
func (s *MySQL) linkObjectTransaction(ctx context.Context, typ string, id, transactionID int64) error {
	_, err := s.CreateEntityRelation(ctx, SaveEntityRelation{SourceType: typ, SourceID: id, TargetType: ObjectTypeTransaction, TargetID: transactionID, RelationType: RelationTypeRelatesTo})
	return err
}
func (s *MySQL) unlinkObjectTransaction(ctx context.Context, typ string, id, transactionID int64) error {
	if !validDomainEventOwner(typ) {
		return fmt.Errorf("unsupported object type")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	firstType, firstID, secondType, secondID := orderedEntities(typ, id, ObjectTypeTransaction, transactionID)
	if err = mysqlValidateAndLockEntity(ctx, tx, firstType, firstID); err != nil {
		return err
	}
	if err = mysqlValidateAndLockEntity(ctx, tx, secondType, secondID); err != nil {
		return err
	}
	r, err := tx.ExecContext(ctx, `delete from entity_relations where source_type=? and source_id=? and target_type='transaction' and target_id=? and relation_type='relates_to'`, typ, id, transactionID)
	if err != nil {
		return err
	}
	n, _ := r.RowsAffected()
	if n == 0 {
		return fmt.Errorf("%s transaction link not found", typ)
	}
	if err = mysqlAppendDomainEvent(ctx, tx, AppendDomainEvent{EntityType: typ, EntityID: id, EventType: "transaction_unlinked", CurrentDate: true, Description: "Transactie ontkoppeld", RelatedEntityType: ObjectTypeTransaction, RelatedEntityID: transactionID}); err != nil {
		return err
	}
	return tx.Commit()
}

func mysqlValidateAndLockEntity(ctx context.Context, tx *sql.Tx, typ string, id int64) error {
	table := ""
	switch typ {
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
	var found int64
	err := tx.QueryRowContext(ctx, `select id from `+table+` where id=? for share`, id).Scan(&found)
	if err == sql.ErrNoRows {
		return fmt.Errorf("%s not found", typ)
	}
	return err
}

func (s *MySQL) appendRelationDomainEvents(ctx context.Context, tx *sql.Tx, v EntityRelation, linked bool) error {
	var date time.Time
	current := true
	if linked {
		switch {
		case v.RelationType == RelationTypeEvidenceFor:
			if err := tx.QueryRowContext(ctx, `select coalesce(document_date,utc_date()) from documents where id=?`, v.SourceID).Scan(&date); err != nil {
				return err
			}
			current = false
		case v.RelationType == RelationTypeRelatesTo && v.TargetType == ObjectTypeTransaction:
			if err := tx.QueryRowContext(ctx, `select booked_on from transactions where id=?`, v.TargetID).Scan(&date); err != nil {
				return err
			}
			current = false
		case v.ValidFrom != "":
			parsed, err := time.Parse("2006-01-02", v.ValidFrom)
			if err != nil {
				return err
			}
			date = parsed
			current = false
		}
	}
	if v.RelationType == RelationTypeEvidenceFor && validDomainEventOwner(v.TargetType) {
		event, description := "document_linked", "Document gekoppeld"
		if !linked {
			event, description = "document_unlinked", "Document ontkoppeld"
		}
		return mysqlAppendDomainEvent(ctx, tx, AppendDomainEvent{EntityType: v.TargetType, EntityID: v.TargetID, EventType: event, EventDate: date, CurrentDate: current, Description: description, RelatedEntityType: ObjectTypeDocument, RelatedEntityID: v.SourceID})
	}
	if v.RelationType == RelationTypeRelatesTo && v.TargetType == ObjectTypeTransaction && validDomainEventOwner(v.SourceType) {
		event, description := "transaction_linked", "Transactie gekoppeld"
		if !linked {
			event, description = "transaction_unlinked", "Transactie ontkoppeld"
		}
		return mysqlAppendDomainEvent(ctx, tx, AppendDomainEvent{EntityType: v.SourceType, EntityID: v.SourceID, EventType: event, EventDate: date, CurrentDate: current, Description: description, RelatedEntityType: ObjectTypeTransaction, RelatedEntityID: v.TargetID})
	}
	objectRelation := v.RelationType == RelationTypeFinancedBy || (v.RelationType == RelationTypeRelatesTo && v.SourceType == ObjectTypeCase && (v.TargetType == ObjectTypeAsset || v.TargetType == ObjectTypeLiability))
	if !objectRelation {
		return nil
	}
	event, description := "relation_linked", "Gerelateerd object gekoppeld"
	if !linked {
		event, description = "relation_unlinked", "Gerelateerd object ontkoppeld"
	}
	if err := mysqlAppendDomainEvent(ctx, tx, AppendDomainEvent{EntityType: v.SourceType, EntityID: v.SourceID, EventType: event, EventDate: date, CurrentDate: current, Description: description, RelatedEntityType: v.TargetType, RelatedEntityID: v.TargetID}); err != nil {
		return err
	}
	return mysqlAppendDomainEvent(ctx, tx, AppendDomainEvent{EntityType: v.TargetType, EntityID: v.TargetID, EventType: event, EventDate: date, CurrentDate: current, Description: description, RelatedEntityType: v.SourceType, RelatedEntityID: v.SourceID})
}

func (s *MySQL) objectTransactions(ctx context.Context, typ string, id int64) ([]Transaction, error) {
	rows, err := s.db.QueryContext(ctx, `select transactions.id,transactions.account_id,coalesce(transactions.category_id,0),date_format(transactions.booked_on,'%Y-%m-%d'),transactions.description,coalesce(parties.name,transactions.counterparty_name_raw,transactions.counterparty,''),coalesce(transactions.counterparty_iban_raw,transactions.counterparty_iban,''),transactions.amount_cents,coalesce(transactions.external_reference,''),coalesce(transactions.import_fingerprint,''),transactions.hidden,(select count(distinct source_id) from entity_relations where source_type='document' and target_type='transaction' and target_id=transactions.id and relation_type='evidence_for') from transactions left join parties on parties.id=transactions.counterparty_party_id where exists(select 1 from entity_relations where target_type='transaction' and target_id=transactions.id and source_type=? and source_id=? and relation_type='relates_to') order by transactions.booked_on desc,transactions.id desc`, typ, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanMySQLTransactions(rows)
}
func (s *MySQL) RelationsForObject(ctx context.Context, typ string, id int64) ([]EntityRelation, error) {
	rows, err := s.db.QueryContext(ctx, `select `+mysqlRelationColumns+` from entity_relations where (source_type=? and source_id=?) or (target_type=? and target_id=?) order by created_at,id`, typ, id, typ, id)
	if err != nil {
		return nil, err
	}
	items := []EntityRelation{}
	for rows.Next() {
		var v EntityRelation
		if err = scanMySQLRelation(rows, &v); err != nil {
			rows.Close()
			return nil, err
		}
		items = append(items, v)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	for i := range items {
		if err = s.addEntityLabels(ctx, &items[i]); err != nil {
			return nil, err
		}
	}
	return items, nil
}
func (s *MySQL) addEntityLabels(ctx context.Context, v *EntityRelation) error {
	source, err := s.entityLabel(ctx, v.SourceType, v.SourceID)
	if err != nil {
		return err
	}
	target, err := s.entityLabel(ctx, v.TargetType, v.TargetID)
	if err != nil {
		return err
	}
	v.SourceLabel, v.TargetLabel = source, target
	return nil
}
func (s *MySQL) entityLabel(ctx context.Context, typ string, id int64) (string, error) {
	var query string
	switch typ {
	case ObjectTypeAsset:
		query = `select name from assets where id=?`
	case ObjectTypeLiability:
		query = `select name from liabilities where id=?`
	case ObjectTypeTransaction:
		query = `select concat(date_format(booked_on,'%Y-%m-%d'),' - ',description) from transactions where id=?`
	case ObjectTypeDocument:
		query = `select original_filename from documents where id=?`
	case ObjectTypeCase:
		query = `select title from cases where id=?`
	case ObjectTypeReservation:
		query = `select name from reservations where id=?`
	default:
		return fmt.Sprintf("%s #%d", typ, id), nil
	}
	var label string
	err := s.db.QueryRowContext(ctx, query, id).Scan(&label)
	if err == sql.ErrNoRows {
		return fmt.Sprintf("%s #%d (niet gevonden)", typ, id), nil
	}
	return label, err
}
