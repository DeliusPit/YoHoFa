package store

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

const mysqlAssetColumns = `id,household_id,asset_type,name,coalesce(description,''),status,coalesce(date_format(acquired_at,'%Y-%m-%d'),''),coalesce(date_format(disposed_at,'%Y-%m-%d'),''),purchase_value,current_value,currency,date_format(created_at,'%Y-%m-%d %H:%i:%s.%f+00'),date_format(updated_at,'%Y-%m-%d %H:%i:%s.%f+00')`
const mysqlLiabilityColumns = `id,household_id,liability_type,name,coalesce(description,''),coalesce(creditor,''),coalesce(date_format(start_date,'%Y-%m-%d'),''),coalesce(date_format(end_date,'%Y-%m-%d'),''),original_amount,current_balance,currency,status,date_format(created_at,'%Y-%m-%d %H:%i:%s.%f+00'),date_format(updated_at,'%Y-%m-%d %H:%i:%s.%f+00')`
const mysqlCaseColumns = `id,household_id,coalesce(main_category_id,0),case_type,title,coalesce(description,''),coalesce(date_format(start_date,'%Y-%m-%d'),''),coalesce(date_format(end_date,'%Y-%m-%d'),''),status,date_format(created_at,'%Y-%m-%d %H:%i:%s.%f+00'),date_format(updated_at,'%Y-%m-%d %H:%i:%s.%f+00')`
const mysqlValuationColumns = `id,asset_id,date_format(valuation_date,'%Y-%m-%d'),value,valuation_type,coalesce(source,''),coalesce(notes,''),date_format(created_at,'%Y-%m-%d %H:%i:%s.%f+00')`
const mysqlDocumentColumns = `id,original_filename,mime_type,size_bytes,sha256,object_key,coalesce(date_format(document_date,'%Y-%m-%d'),''),coalesce(description,''),date_format(created_at,'%Y-%m-%d %H:%i:%s.%f+00')`

func (s *MySQL) CreateDocument(ctx context.Context, input CreateDocument) (Document, error) {
	var date any
	if input.DocumentDate != "" {
		v, err := time.Parse("2006-01-02", input.DocumentDate)
		if err != nil {
			return Document{}, fmt.Errorf("invalid document date")
		}
		date = v
	}
	r, err := s.db.ExecContext(ctx, `insert into documents(original_filename,mime_type,size_bytes,sha256,object_key,document_date,description) values (?,?,?,?,?,?,?)`, input.OriginalFilename, input.MIMEType, input.SizeBytes, input.SHA256, input.ObjectKey, date, input.Description)
	if err != nil {
		return Document{}, err
	}
	id, err := r.LastInsertId()
	if err != nil {
		return Document{}, err
	}
	return s.Document(ctx, id)
}
func (s *MySQL) Document(ctx context.Context, id int64) (Document, error) {
	var v Document
	err := s.db.QueryRowContext(ctx, `select `+mysqlDocumentColumns+` from documents where id=?`, id).Scan(&v.ID, &v.OriginalFilename, &v.MIMEType, &v.SizeBytes, &v.SHA256, &v.ObjectKey, &v.DocumentDate, &v.Description, &v.CreatedAt)
	if err == sql.ErrNoRows {
		return Document{}, fmt.Errorf("document not found")
	}
	return v, err
}
func (s *MySQL) Documents(ctx context.Context) ([]Document, error) {
	return s.documentsQuery(ctx, `select `+mysqlDocumentColumns+` from documents order by created_at desc,id desc`)
}
func (s *MySQL) documentsQuery(ctx context.Context, q string, args ...any) ([]Document, error) {
	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []Document{}
	for rows.Next() {
		var v Document
		if err := rows.Scan(&v.ID, &v.OriginalFilename, &v.MIMEType, &v.SizeBytes, &v.SHA256, &v.ObjectKey, &v.DocumentDate, &v.Description, &v.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, v)
	}
	return items, rows.Err()
}
func (s *MySQL) LinkDocument(ctx context.Context, documentID int64, objectType string, objectID int64) error {
	if !supportedDocumentTarget(objectType) {
		return fmt.Errorf("unsupported object type")
	}
	_, err := s.CreateEntityRelation(ctx, SaveEntityRelation{SourceType: ObjectTypeDocument, SourceID: documentID, TargetType: objectType, TargetID: objectID, RelationType: RelationTypeEvidenceFor})
	return err
}
func (s *MySQL) DocumentsForObject(ctx context.Context, objectType string, objectID int64) ([]Document, error) {
	if !supportedDocumentTarget(objectType) {
		return nil, fmt.Errorf("unsupported object type")
	}
	return s.documentsQuery(ctx, `select `+mysqlDocumentColumns+` from documents where exists(select 1 from entity_relations where source_type='document' and source_id=documents.id and target_type=? and target_id=? and relation_type='evidence_for') order by created_at desc,id desc`, objectType, objectID)
}
func (s *MySQL) UnlinkDocument(ctx context.Context, documentID int64, objectType string, objectID int64) error {
	if !supportedDocumentTarget(objectType) {
		return fmt.Errorf("unsupported object type")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	firstType, firstID, secondType, secondID := orderedEntities(ObjectTypeDocument, documentID, objectType, objectID)
	if err = mysqlValidateAndLockEntity(ctx, tx, firstType, firstID); err != nil {
		return err
	}
	if err = mysqlValidateAndLockEntity(ctx, tx, secondType, secondID); err != nil {
		return err
	}
	r, err := tx.ExecContext(ctx, `delete from entity_relations where source_type='document' and source_id=? and target_type=? and target_id=? and relation_type='evidence_for'`, documentID, objectType, objectID)
	if err != nil {
		return err
	}
	n, _ := r.RowsAffected()
	if n == 0 {
		return fmt.Errorf("document link not found")
	}
	if validDomainEventOwner(objectType) {
		if err = mysqlAppendDomainEvent(ctx, tx, AppendDomainEvent{EntityType: objectType, EntityID: objectID, EventType: "document_unlinked", CurrentDate: true, Description: "Document ontkoppeld", RelatedEntityType: ObjectTypeDocument, RelatedEntityID: documentID}); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *MySQL) Assets(ctx context.Context, filter AssetFilter) ([]Asset, error) {
	rows, err := s.db.QueryContext(ctx, `select `+mysqlAssetColumns+` from assets where (?='' or asset_type=?) and (?='' or status=?) order by status,name,id`, filter.AssetType, filter.AssetType, filter.Status, filter.Status)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []Asset{}
	for rows.Next() {
		var v Asset
		if err := scanAsset(rows, &v); err != nil {
			return nil, err
		}
		items = append(items, v)
	}
	return items, rows.Err()
}
func (s *MySQL) Asset(ctx context.Context, id int64) (Asset, error) {
	var v Asset
	err := scanAsset(s.db.QueryRowContext(ctx, `select `+mysqlAssetColumns+` from assets where id=?`, id), &v)
	if err == sql.ErrNoRows {
		return Asset{}, fmt.Errorf("asset not found")
	}
	return v, err
}
func (s *MySQL) CreateAsset(ctx context.Context, input SaveAsset) (Asset, error) {
	input, acquired, disposed, err := normalizeAssetInput(input)
	if err != nil {
		return Asset{}, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Asset{}, err
	}
	defer tx.Rollback()
	r, err := tx.ExecContext(ctx, `insert into assets(household_id,asset_type,name,description,status,acquired_at,disposed_at,purchase_value,currency) values ((select id from households order by id limit 1),?,?,?,?,?,?,?,?)`, input.AssetType, input.Name, input.Description, input.Status, acquired, disposed, input.PurchaseValueCents, input.Currency)
	if err != nil {
		return Asset{}, err
	}
	id, err := r.LastInsertId()
	if err != nil {
		return Asset{}, err
	}
	if err = mysqlAppendDomainEvent(ctx, tx, AppendDomainEvent{EntityType: ObjectTypeAsset, EntityID: id, EventType: "asset_created", CurrentDate: true, Description: "Bezitting aangemaakt"}); err != nil {
		return Asset{}, err
	}
	var v Asset
	if err = scanAsset(tx.QueryRowContext(ctx, `select `+mysqlAssetColumns+` from assets where id=?`, id), &v); err != nil {
		return Asset{}, err
	}
	if err = tx.Commit(); err != nil {
		return Asset{}, err
	}
	return v, nil
}
func (s *MySQL) UpdateAsset(ctx context.Context, id int64, input SaveAsset) (Asset, error) {
	input, acquired, disposed, err := normalizeAssetInput(input)
	if err != nil {
		return Asset{}, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Asset{}, err
	}
	defer tx.Rollback()
	var currency string
	var purchase int64
	if err = tx.QueryRowContext(ctx, `select currency,purchase_value from assets where id=? for update`, id).Scan(&currency, &purchase); err == sql.ErrNoRows {
		return Asset{}, fmt.Errorf("asset not found")
	} else if err != nil {
		return Asset{}, err
	}
	if strings.TrimSpace(currency) != input.Currency {
		var count int
		if err = tx.QueryRowContext(ctx, `select count(*) from asset_valuations where asset_id=?`, id).Scan(&count); err != nil {
			return Asset{}, err
		}
		if count > 0 || purchase > 0 {
			return Asset{}, fmt.Errorf("currency cannot change while asset values exist")
		}
	}
	_, err = tx.ExecContext(ctx, `update assets set asset_type=?,name=?,description=?,status=?,acquired_at=?,disposed_at=?,purchase_value=?,currency=?,updated_at=utc_timestamp(6) where id=?`, input.AssetType, input.Name, input.Description, input.Status, acquired, disposed, input.PurchaseValueCents, input.Currency, id)
	if err != nil {
		return Asset{}, err
	}
	if err = mysqlAppendDomainEvent(ctx, tx, AppendDomainEvent{EntityType: ObjectTypeAsset, EntityID: id, EventType: "asset_updated", CurrentDate: true, Description: "Bezitting bijgewerkt"}); err != nil {
		return Asset{}, err
	}
	var v Asset
	if err = scanAsset(tx.QueryRowContext(ctx, `select `+mysqlAssetColumns+` from assets where id=?`, id), &v); err != nil {
		return Asset{}, err
	}
	if err = tx.Commit(); err != nil {
		return Asset{}, err
	}
	return v, nil
}
func (s *MySQL) DeleteAsset(ctx context.Context, id int64) error {
	return s.deleteDomainObject(ctx, ObjectTypeAsset, id)
}

func (s *MySQL) Liabilities(ctx context.Context, filter LiabilityFilter) ([]Liability, error) {
	rows, err := s.db.QueryContext(ctx, `select `+mysqlLiabilityColumns+` from liabilities where (?='' or liability_type=?) and (?='' or status=?) order by status,name,id`, filter.LiabilityType, filter.LiabilityType, filter.Status, filter.Status)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []Liability{}
	for rows.Next() {
		var v Liability
		if err := scanLiability(rows, &v); err != nil {
			return nil, err
		}
		items = append(items, v)
	}
	return items, rows.Err()
}
func (s *MySQL) Liability(ctx context.Context, id int64) (Liability, error) {
	var v Liability
	err := scanLiability(s.db.QueryRowContext(ctx, `select `+mysqlLiabilityColumns+` from liabilities where id=?`, id), &v)
	if err == sql.ErrNoRows {
		return Liability{}, fmt.Errorf("liability not found")
	}
	return v, err
}
func (s *MySQL) CreateLiability(ctx context.Context, input SaveLiability) (Liability, error) {
	input, start, end, err := normalizeLiabilityInput(input)
	if err != nil {
		return Liability{}, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Liability{}, err
	}
	defer tx.Rollback()
	r, err := tx.ExecContext(ctx, `insert into liabilities(household_id,liability_type,name,description,creditor,start_date,end_date,original_amount,current_balance,currency,status) values ((select id from households order by id limit 1),?,?,?,?,?,?,?,?,?,?)`, input.LiabilityType, input.Name, input.Description, input.Creditor, start, end, input.OriginalAmountCents, input.CurrentBalanceCents, input.Currency, input.Status)
	if err != nil {
		return Liability{}, err
	}
	id, err := r.LastInsertId()
	if err != nil {
		return Liability{}, err
	}
	if err = mysqlAppendDomainEvent(ctx, tx, AppendDomainEvent{EntityType: ObjectTypeLiability, EntityID: id, EventType: "liability_created", CurrentDate: true, Description: "Schuld aangemaakt"}); err != nil {
		return Liability{}, err
	}
	var v Liability
	if err = scanLiability(tx.QueryRowContext(ctx, `select `+mysqlLiabilityColumns+` from liabilities where id=?`, id), &v); err != nil {
		return Liability{}, err
	}
	if err = tx.Commit(); err != nil {
		return Liability{}, err
	}
	return v, nil
}
func (s *MySQL) UpdateLiability(ctx context.Context, id int64, input SaveLiability) (Liability, error) {
	input, start, end, err := normalizeLiabilityInput(input)
	if err != nil {
		return Liability{}, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Liability{}, err
	}
	defer tx.Rollback()
	var old string
	if err = tx.QueryRowContext(ctx, `select status from liabilities where id=? for update`, id).Scan(&old); err == sql.ErrNoRows {
		return Liability{}, fmt.Errorf("liability not found")
	} else if err != nil {
		return Liability{}, err
	}
	_, err = tx.ExecContext(ctx, `update liabilities set liability_type=?,name=?,description=?,creditor=?,start_date=?,end_date=?,original_amount=?,current_balance=?,currency=?,status=?,updated_at=utc_timestamp(6) where id=?`, input.LiabilityType, input.Name, input.Description, input.Creditor, start, end, input.OriginalAmountCents, input.CurrentBalanceCents, input.Currency, input.Status, id)
	if err != nil {
		return Liability{}, err
	}
	event, description := "liability_updated", "Schuld bijgewerkt"
	if old != "repaid" && input.Status == "repaid" {
		event, description = "liability_repaid", "Schuld afgelost"
	}
	if err = mysqlAppendDomainEvent(ctx, tx, AppendDomainEvent{EntityType: ObjectTypeLiability, EntityID: id, EventType: event, CurrentDate: true, Description: description}); err != nil {
		return Liability{}, err
	}
	var v Liability
	if err = scanLiability(tx.QueryRowContext(ctx, `select `+mysqlLiabilityColumns+` from liabilities where id=?`, id), &v); err != nil {
		return Liability{}, err
	}
	if err = tx.Commit(); err != nil {
		return Liability{}, err
	}
	return v, nil
}
func (s *MySQL) DeleteLiability(ctx context.Context, id int64) error {
	return s.deleteDomainObject(ctx, ObjectTypeLiability, id)
}

func (s *MySQL) Cases(ctx context.Context, filter CaseFilter) ([]Case, error) {
	filter.CaseType = strings.TrimSpace(filter.CaseType)
	filter.Status = strings.TrimSpace(filter.Status)
	if filter.Status != "" && !validCaseStatus(filter.Status) {
		return nil, fmt.Errorf("invalid case status")
	}
	rows, err := s.db.QueryContext(ctx, `select `+mysqlCaseColumns+` from cases where (?='' or case_type=?) and (?='' or status=?) order by status,title,id`, filter.CaseType, filter.CaseType, filter.Status, filter.Status)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []Case{}
	for rows.Next() {
		var v Case
		if err := scanCase(rows, &v); err != nil {
			return nil, err
		}
		items = append(items, v)
	}
	return items, rows.Err()
}
func (s *MySQL) Case(ctx context.Context, id int64) (Case, error) {
	var v Case
	err := scanCase(s.db.QueryRowContext(ctx, `select `+mysqlCaseColumns+` from cases where id=?`, id), &v)
	if err == sql.ErrNoRows {
		return Case{}, fmt.Errorf("case not found")
	}
	return v, err
}
func (s *MySQL) CreateCase(ctx context.Context, input SaveCase) (Case, error) {
	input, start, end, err := normalizeCaseInput(input)
	if err != nil {
		return Case{}, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Case{}, err
	}
	defer tx.Rollback()
	r, err := tx.ExecContext(ctx, `insert into cases(household_id,case_type,title,description,start_date,end_date,status) values ((select id from households order by id limit 1),?,?,?,?,?,?)`, input.CaseType, input.Title, input.Description, start, end, input.Status)
	if err != nil {
		return Case{}, err
	}
	id, err := r.LastInsertId()
	if err != nil {
		return Case{}, err
	}
	if err = mysqlAppendDomainEvent(ctx, tx, AppendDomainEvent{EntityType: ObjectTypeCase, EntityID: id, EventType: "case_created", CurrentDate: true, Description: "Dossier aangemaakt"}); err != nil {
		return Case{}, err
	}
	var v Case
	if err = scanCase(tx.QueryRowContext(ctx, `select `+mysqlCaseColumns+` from cases where id=?`, id), &v); err != nil {
		return Case{}, err
	}
	if err = tx.Commit(); err != nil {
		return Case{}, err
	}
	return v, nil
}
func (s *MySQL) UpdateCase(ctx context.Context, id int64, input SaveCase) (Case, error) {
	input, start, end, err := normalizeCaseInput(input)
	if err != nil {
		return Case{}, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Case{}, err
	}
	defer tx.Rollback()
	var old string
	var mainCategoryID int64
	if err = tx.QueryRowContext(ctx, `select status,coalesce(main_category_id,0) from cases where id=? for update`, id).Scan(&old, &mainCategoryID); err == sql.ErrNoRows {
		return Case{}, fmt.Errorf("case not found")
	} else if err != nil {
		return Case{}, err
	}
	if mainCategoryID != 0 {
		return Case{}, fmt.Errorf("case is managed by a main category")
	}
	_, err = tx.ExecContext(ctx, `update cases set case_type=?,title=?,description=?,start_date=?,end_date=?,status=?,updated_at=utc_timestamp(6) where id=?`, input.CaseType, input.Title, input.Description, start, end, input.Status, id)
	if err != nil {
		return Case{}, err
	}
	event, description := "case_updated", "Dossier bijgewerkt"
	if old != "completed" && input.Status == "completed" {
		event, description = "case_completed", "Dossier afgerond"
	}
	if err = mysqlAppendDomainEvent(ctx, tx, AppendDomainEvent{EntityType: ObjectTypeCase, EntityID: id, EventType: event, CurrentDate: true, Description: description}); err != nil {
		return Case{}, err
	}
	var v Case
	if err = scanCase(tx.QueryRowContext(ctx, `select `+mysqlCaseColumns+` from cases where id=?`, id), &v); err != nil {
		return Case{}, err
	}
	if err = tx.Commit(); err != nil {
		return Case{}, err
	}
	return v, nil
}
func (s *MySQL) DeleteCase(ctx context.Context, id int64) error {
	return s.deleteDomainObject(ctx, ObjectTypeCase, id)
}

func (s *MySQL) deleteDomainObject(ctx context.Context, typ string, id int64) error {
	table, label := typ+"s", typ
	if typ == ObjectTypeLiability {
		table = "liabilities"
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var locked int64
	if err = tx.QueryRowContext(ctx, `select id from `+table+` where id=? for update`, id).Scan(&locked); err == sql.ErrNoRows {
		return fmt.Errorf("%s not found", label)
	} else if err != nil {
		return err
	}
	if typ == ObjectTypeCase {
		var mainCategoryID int64
		if err = tx.QueryRowContext(ctx, `select coalesce(main_category_id,0) from cases where id=?`, id).Scan(&mainCategoryID); err != nil {
			return err
		}
		if mainCategoryID != 0 {
			return fmt.Errorf("case is managed by a main category")
		}
	}
	var documents, relations int
	if err = tx.QueryRowContext(ctx, `select count(*) from entity_relations where source_type='document' and target_type=? and target_id=? and relation_type='evidence_for'`, typ, id).Scan(&documents); err != nil {
		return err
	}
	if err = tx.QueryRowContext(ctx, `select count(*) from entity_relations where ((source_type=? and source_id=?) or (target_type=? and target_id=?)) and not(source_type='document' and target_type=? and target_id=? and relation_type='evidence_for')`, typ, id, typ, id, typ, id).Scan(&relations); err != nil {
		return err
	}
	if documents > 0 || relations > 0 {
		return fmt.Errorf("%s still has %d document links and %d object relations", typ, documents, relations)
	}
	if err = mysqlCleanupDomainEvents(ctx, tx, typ, id); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `delete from `+table+` where id=?`, id); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *MySQL) AssetValuations(ctx context.Context, assetID int64) ([]AssetValuation, error) {
	rows, err := s.db.QueryContext(ctx, `select `+mysqlValuationColumns+` from asset_valuations where asset_id=? order by valuation_date desc,id desc`, assetID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []AssetValuation{}
	for rows.Next() {
		var v AssetValuation
		if err := scanAssetValuation(rows, &v); err != nil {
			return nil, err
		}
		items = append(items, v)
	}
	return items, rows.Err()
}
func (s *MySQL) CreateAssetValuation(ctx context.Context, assetID int64, input SaveAssetValuation) (AssetValuation, error) {
	input, date, err := normalizeAssetValuation(input)
	if err != nil {
		return AssetValuation{}, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return AssetValuation{}, err
	}
	defer tx.Rollback()
	if err = mysqlLockAsset(ctx, tx, assetID); err != nil {
		return AssetValuation{}, err
	}
	r, err := tx.ExecContext(ctx, `insert into asset_valuations(asset_id,valuation_date,value,valuation_type,source,notes) values (?,?,?,?,?,?)`, assetID, date, input.ValueCents, input.ValuationType, input.Source, input.Notes)
	if err != nil {
		return AssetValuation{}, err
	}
	id, err := r.LastInsertId()
	if err != nil {
		return AssetValuation{}, err
	}
	if err = mysqlUpdateAssetValue(ctx, tx, assetID); err != nil {
		return AssetValuation{}, err
	}
	if err = mysqlAppendDomainEvent(ctx, tx, AppendDomainEvent{EntityType: ObjectTypeAsset, EntityID: assetID, EventType: "valuation_added", EventDate: date, Description: "Waardering toegevoegd"}); err != nil {
		return AssetValuation{}, err
	}
	var v AssetValuation
	if err = scanAssetValuation(tx.QueryRowContext(ctx, `select `+mysqlValuationColumns+` from asset_valuations where id=?`, id), &v); err != nil {
		return AssetValuation{}, err
	}
	if err = tx.Commit(); err != nil {
		return AssetValuation{}, err
	}
	return v, nil
}
func (s *MySQL) UpdateAssetValuation(ctx context.Context, assetID, id int64, input SaveAssetValuation) (AssetValuation, error) {
	input, date, err := normalizeAssetValuation(input)
	if err != nil {
		return AssetValuation{}, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return AssetValuation{}, err
	}
	defer tx.Rollback()
	if err = mysqlLockAsset(ctx, tx, assetID); err != nil {
		return AssetValuation{}, err
	}
	r, err := tx.ExecContext(ctx, `update asset_valuations set valuation_date=?,value=?,valuation_type=?,source=?,notes=? where id=? and asset_id=?`, date, input.ValueCents, input.ValuationType, input.Source, input.Notes, id, assetID)
	if err != nil {
		return AssetValuation{}, err
	}
	n, _ := r.RowsAffected()
	if n == 0 {
		return AssetValuation{}, fmt.Errorf("asset valuation not found")
	}
	if err = mysqlUpdateAssetValue(ctx, tx, assetID); err != nil {
		return AssetValuation{}, err
	}
	if err = mysqlAppendDomainEvent(ctx, tx, AppendDomainEvent{EntityType: ObjectTypeAsset, EntityID: assetID, EventType: "valuation_updated", EventDate: date, Description: "Waardering bijgewerkt"}); err != nil {
		return AssetValuation{}, err
	}
	var v AssetValuation
	if err = scanAssetValuation(tx.QueryRowContext(ctx, `select `+mysqlValuationColumns+` from asset_valuations where id=?`, id), &v); err != nil {
		return AssetValuation{}, err
	}
	if err = tx.Commit(); err != nil {
		return AssetValuation{}, err
	}
	return v, nil
}
func (s *MySQL) DeleteAssetValuation(ctx context.Context, assetID, id int64) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err = mysqlLockAsset(ctx, tx, assetID); err != nil {
		return err
	}
	r, err := tx.ExecContext(ctx, `delete from asset_valuations where id=? and asset_id=?`, id, assetID)
	if err != nil {
		return err
	}
	n, _ := r.RowsAffected()
	if n == 0 {
		return fmt.Errorf("asset valuation not found")
	}
	if err = mysqlUpdateAssetValue(ctx, tx, assetID); err != nil {
		return err
	}
	if err = mysqlAppendDomainEvent(ctx, tx, AppendDomainEvent{EntityType: ObjectTypeAsset, EntityID: assetID, EventType: "valuation_removed", CurrentDate: true, Description: "Waardering verwijderd"}); err != nil {
		return err
	}
	return tx.Commit()
}
func mysqlLockAsset(ctx context.Context, tx *sql.Tx, id int64) error {
	var v int64
	err := tx.QueryRowContext(ctx, `select id from assets where id=? for update`, id).Scan(&v)
	if err == sql.ErrNoRows {
		return fmt.Errorf("asset not found")
	}
	return err
}
func mysqlUpdateAssetValue(ctx context.Context, tx *sql.Tx, id int64) error {
	_, err := tx.ExecContext(ctx, `update assets set current_value=coalesce((select value from asset_valuations where asset_id=? order by valuation_date desc,id desc limit 1),0),updated_at=utc_timestamp(6) where id=?`, id, id)
	return err
}

func mysqlAppendDomainEvent(ctx context.Context, tx *sql.Tx, input AppendDomainEvent) error {
	input.EntityType = strings.TrimSpace(input.EntityType)
	input.EventType = strings.TrimSpace(input.EventType)
	input.Description = strings.TrimSpace(input.Description)
	input.RelatedEntityType = strings.TrimSpace(input.RelatedEntityType)
	if err := validateAppendDomainEvent(input); err != nil {
		return err
	}
	var relatedType, relatedID, eventDate any
	if input.RelatedEntityType != "" {
		relatedType, relatedID = input.RelatedEntityType, input.RelatedEntityID
	}
	if !input.CurrentDate {
		eventDate = input.EventDate
	}
	_, err := tx.ExecContext(ctx, `insert into domain_events(entity_type,entity_id,event_type,event_date,description,related_entity_type,related_entity_id) values (?,?,?,coalesce(?,utc_date()),?,?,?)`, input.EntityType, input.EntityID, input.EventType, eventDate, input.Description, relatedType, relatedID)
	return err
}
func mysqlCleanupDomainEvents(ctx context.Context, tx *sql.Tx, typ string, id int64) error {
	if _, err := tx.ExecContext(ctx, `delete from domain_events where entity_type=? and entity_id=?`, typ, id); err != nil {
		return err
	}
	_, err := tx.ExecContext(ctx, `update domain_events set related_entity_type=null,related_entity_id=null where related_entity_type=? and related_entity_id=?`, typ, id)
	return err
}
func (s *MySQL) DomainEventsForObject(ctx context.Context, typ string, id int64) ([]DomainEvent, error) {
	if !validDomainEventOwner(typ) || id <= 0 {
		return nil, fmt.Errorf("invalid domain event entity")
	}
	if err := s.mysqlOwnerExists(ctx, typ, id); err != nil {
		return nil, err
	}
	rows, err := s.db.QueryContext(ctx, `select id,entity_type,entity_id,event_type,date_format(event_date,'%Y-%m-%d'),description,related_entity_type,related_entity_id,date_format(created_at,'%Y-%m-%d %H:%i:%s.%f+00') from domain_events where entity_type=? and entity_id=? order by event_date desc,id desc limit 200`, typ, id)
	if err != nil {
		return nil, err
	}
	items := []DomainEvent{}
	for rows.Next() {
		var v DomainEvent
		if err = rows.Scan(&v.ID, &v.EntityType, &v.EntityID, &v.EventType, &v.EventDate, &v.Description, &v.RelatedEntityType, &v.RelatedEntityID, &v.CreatedAt); err != nil {
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
		if items[i].RelatedEntityType != nil {
			label, e := s.entityLabel(ctx, *items[i].RelatedEntityType, *items[i].RelatedEntityID)
			if e != nil {
				return nil, e
			}
			items[i].RelatedEntityLabel = &label
		}
	}
	return items, nil
}
func (s *MySQL) mysqlOwnerExists(ctx context.Context, typ string, id int64) error {
	table := typ + "s"
	if typ == ObjectTypeLiability {
		table = "liabilities"
	}
	var exists bool
	if err := s.db.QueryRowContext(ctx, `select exists(select 1 from `+table+` where id=?)`, id).Scan(&exists); err != nil {
		return err
	}
	if !exists {
		return fmt.Errorf("%s not found", typ)
	}
	return nil
}
