package store

import (
	"context"
	"database/sql"
	_ "embed"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"modernc.org/sqlite"
)

const CoreSchemaVersion = 5

//go:embed core_schema.sql
var coreSchema string

var (
	ErrCoreInvalid  = errors.New("Ongeldige invoer")
	ErrCoreNotFound = errors.New("Niet gevonden")
	ErrCoreConflict = errors.New("De gegevens zijn gekoppeld of bestaan al")
)

// Core is the active v3 store. It never invokes the legacy schema or seeds.
type Core struct {
	db *sql.DB
}

type FinancialObject struct {
	ID           int64        `json:"id"`
	ObjectType   string       `json:"objectType"`
	Name         string       `json:"name"`
	Description  string       `json:"description"`
	CurrentValue *ObjectValue `json:"currentValue,omitempty"`
}

type SaveFinancialObject struct {
	ObjectType  string `json:"objectType"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

type ObjectValue struct {
	ID                int64  `json:"id"`
	FinancialObjectID int64  `json:"financialObjectId"`
	ValueDate         string `json:"valueDate"`
	ValueCents        int64  `json:"valueCents"`
}

type SaveObjectValue struct {
	ValueDate  string `json:"valueDate"`
	ValueCents *int64 `json:"valueCents"`
}

type ObjectRelation struct {
	SourceObjectID int64  `json:"sourceObjectId"`
	TargetObjectID int64  `json:"targetObjectId"`
	RelationType   string `json:"relationType"`
}

type CoreSummary struct {
	SchemaVersion    int `json:"schemaVersion"`
	FinancialObjects int `json:"financialObjects"`
	ObjectValues     int `json:"objectValues"`
	ObjectRelations  int `json:"objectRelations"`
	Transactions     int `json:"transactions"`
	Documents        int `json:"documents"`
	ImportProfiles   int `json:"importProfiles"`
}

func OpenCore(ctx context.Context, path string) (*Core, error) {
	if strings.TrimSpace(path) == "" || strings.ContainsAny(path, "?\x00") || strings.HasPrefix(path, "file:") || path == ":memory:" {
		return nil, fmt.Errorf("%w: kies een SQLite-bestand", ErrCoreInvalid)
	}
	absolute, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(absolute), 0o700); err != nil {
		return nil, err
	}
	query := url.Values{}
	// modernc applies these pragmas to every newly opened connection.
	query.Add("_pragma", "foreign_keys(1)")
	query.Add("_pragma", "busy_timeout(5000)")
	dsn := (&url.URL{Scheme: "file", Path: absolute, RawQuery: query.Encode()}).String()
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	s := &Core{db: db}
	if err := s.initialize(ctx); err != nil {
		db.Close()
		return nil, err
	}
	if _, err := db.ExecContext(ctx, "PRAGMA journal_mode = WAL"); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}

func (s *Core) initialize(ctx context.Context) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var version int
	if err := tx.QueryRowContext(ctx, "PRAGMA user_version").Scan(&version); err != nil {
		return err
	}
	if version > CoreSchemaVersion {
		return fmt.Errorf("De databaseversie is nieuwer dan deze applicatie")
	}
	rows, err := tx.QueryContext(ctx, "SELECT name FROM sqlite_master WHERE type = 'table' AND name NOT LIKE 'sqlite_%' ORDER BY name")
	if err != nil {
		return err
	}
	var names []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			rows.Close()
			return err
		}
		names = append(names, name)
	}
	rowErr := rows.Err()
	rows.Close()
	if rowErr != nil {
		return rowErr
	}
	if version == 0 {
		if len(names) != 0 {
			return fmt.Errorf("Deze database bevat een oud schema; kies een nieuw DATABASE_PATH voor v3")
		}
		if _, err := tx.ExecContext(ctx, coreSchema); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, "PRAGMA user_version = "+strconv.Itoa(CoreSchemaVersion)); err != nil {
			return err
		}
	} else {
		expected := []string{"bank_import_profiles", "bank_import_rows", "document_financial_objects", "documents", "financial_objects", "object_relations", "object_values", "transaction_financial_objects", "transactions"}
		if !slices.Equal(names, expected) {
			return fmt.Errorf("Het databaseschema komt niet overeen met v3")
		}
		if version == 1 {
			if _, err := tx.ExecContext(ctx, "ALTER TABLE transactions ADD COLUMN hidden INTEGER NOT NULL DEFAULT 0 CHECK (hidden IN (0, 1))"); err != nil {
				return err
			}
			if _, err := tx.ExecContext(ctx, "PRAGMA user_version = 2"); err != nil {
				return err
			}
		}
		if version < 3 {
			if _, err := tx.ExecContext(ctx, "ALTER TABLE transactions ADD COLUMN status TEXT NOT NULL DEFAULT 'I' CHECK (status IN ('I', 'A', 'F', 'U'))"); err != nil {
				return err
			}
			if _, err := tx.ExecContext(ctx, "UPDATE transactions SET status = CASE WHEN EXISTS (SELECT 1 FROM transaction_financial_objects l WHERE l.transaction_id = transactions.id) THEN 'U' ELSE 'I' END"); err != nil {
				return err
			}
		}
		docColumns, err := tableColumns(ctx, tx, "documents")
		if err != nil {
			return err
		}
		if !docColumns["mime_type"] {
			// SQLite staat geen niet-constante default bij ALTER toe; de kolom krijgt
			// een lege default en wordt daarna teruggevuld met het actuele tijdstip.
			for _, statement := range []string{
				"ALTER TABLE documents ADD COLUMN mime_type TEXT NOT NULL DEFAULT ''",
				"ALTER TABLE documents ADD COLUMN size_bytes INTEGER NOT NULL DEFAULT 0 CHECK (size_bytes >= 0)",
				"ALTER TABLE documents ADD COLUMN sha256 TEXT NOT NULL DEFAULT ''",
				"ALTER TABLE documents ADD COLUMN created_at TEXT NOT NULL DEFAULT ''",
				"UPDATE documents SET created_at = strftime('%Y-%m-%dT%H:%M:%SZ', 'now') WHERE created_at = ''",
			} {
				if _, err := tx.ExecContext(ctx, statement); err != nil {
					return err
				}
			}
		}
		if _, err := tx.ExecContext(ctx, "CREATE UNIQUE INDEX IF NOT EXISTS documents_object_key_uidx ON documents(object_key)"); err != nil {
			return err
		}
		if version < 4 {
			if err := rebuildCoreTablesForForeignKeys(ctx, tx); err != nil {
				return err
			}
		}
		if version < 5 {
			if _, err := tx.ExecContext(ctx, "ALTER TABLE documents ADD COLUMN status TEXT NOT NULL DEFAULT 'I' CHECK (status IN ('I', 'A', 'F', 'U'))"); err != nil {
				return err
			}
		}
		if version != CoreSchemaVersion {
			if _, err := tx.ExecContext(ctx, "PRAGMA user_version = "+strconv.Itoa(CoreSchemaVersion)); err != nil {
				return err
			}
		}
	}
	return tx.Commit()
}

func tableColumns(ctx context.Context, tx *sql.Tx, table string) (map[string]bool, error) {
	rows, err := tx.QueryContext(ctx, "PRAGMA table_info("+table+")")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	columns := map[string]bool{}
	for rows.Next() {
		var cid int
		var name, ctype string
		var notnull, pk int
		var dfltValue any
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dfltValue, &pk); err != nil {
			return nil, err
		}
		columns[name] = true
	}
	return columns, rows.Err()
}

// rebuildCoreTablesForForeignKeys geeft de koppeltabellen de cascade-regels
// uit het ingebedde schema. ALTER TABLE kan bestaande FK-clausules niet
// wijzigen, dus worden de tabellen opnieuw opgebouwd met exact de definitie
// uit core_schema.sql. De definities worden daar uitgelezen, zodat schema en
// migratie niet uit elkaar kunnen lopen.
func rebuildCoreTablesForForeignKeys(ctx context.Context, tx *sql.Tx) error {
	tables := []string{"transaction_financial_objects", "document_financial_objects", "object_values", "bank_import_rows", "object_relations"}
	definitions := map[string]string{}
	for _, statement := range strings.Split(coreSchema, ";") {
		trimmed := strings.TrimSpace(statement)
		if !strings.HasPrefix(trimmed, "CREATE TABLE") {
			continue
		}
		for _, table := range tables {
			if strings.HasPrefix(trimmed, "CREATE TABLE "+table+" (") {
				definitions[table] = trimmed
			}
		}
	}
	if len(definitions) != len(tables) {
		return fmt.Errorf("Het ingebedde schema mist een tabeldefinitie voor de foreign key-migratie")
	}
	if _, err := tx.ExecContext(ctx, "PRAGMA foreign_keys = OFF"); err != nil {
		return err
	}
	defer tx.ExecContext(ctx, "PRAGMA foreign_keys = ON")
	for _, table := range tables {
		if _, err := tx.ExecContext(ctx, "ALTER TABLE "+table+" RENAME TO "+table+"__v4"); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, definitions[table]); err != nil {
			return err
		}
		columns, err := tableColumns(ctx, tx, table+"__v4")
		if err != nil {
			return err
		}
		columnList := make([]string, 0, len(columns))
		for column := range columns {
			columnList = append(columnList, column)
		}
		slices.Sort(columnList)
		query := "INSERT INTO " + table + " (" + strings.Join(columnList, ", ") + ") SELECT " + strings.Join(columnList, ", ") + " FROM " + table + "__v4"
		if _, err := tx.ExecContext(ctx, query); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, "DROP TABLE "+table+"__v4"); err != nil {
			return err
		}
	}
	if _, err := tx.ExecContext(ctx, "PRAGMA foreign_key_check"); err != nil {
		return err
	}
	return nil
}

func (s *Core) Close() { _ = s.db.Close() }

func (s *Core) Ping(ctx context.Context) error { return s.db.PingContext(ctx) }

func (s *Core) CoreSummary(ctx context.Context) (CoreSummary, error) {
	item := CoreSummary{SchemaVersion: CoreSchemaVersion}
	err := s.db.QueryRowContext(ctx, `SELECT
		(SELECT count(*) FROM financial_objects), (SELECT count(*) FROM object_values),
		(SELECT count(*) FROM object_relations), (SELECT count(*) FROM transactions),
		(SELECT count(*) FROM documents), (SELECT count(*) FROM bank_import_profiles)`).Scan(
		&item.FinancialObjects, &item.ObjectValues, &item.ObjectRelations, &item.Transactions, &item.Documents, &item.ImportProfiles)
	return item, err
}

func validFinancialObjectType(value string) bool {
	return value == "asset" || value == "debt" || value == "expense" || value == "reserve"
}

func normalizeFinancialObject(input SaveFinancialObject) (SaveFinancialObject, error) {
	input.ObjectType = strings.TrimSpace(input.ObjectType)
	input.Name = strings.TrimSpace(input.Name)
	input.Description = strings.TrimSpace(input.Description)
	if !validFinancialObjectType(input.ObjectType) || input.Name == "" {
		return input, fmt.Errorf("%w: kies een van de vier objecttypes en vul een naam in", ErrCoreInvalid)
	}
	return input, nil
}

const financialObjectSelect = `SELECT o.id, o.object_type, o.name, coalesce(o.description, ''), v.id, v.value_date, v.value_cents
	FROM financial_objects o LEFT JOIN object_values v ON v.id = (
		SELECT id FROM object_values WHERE financial_object_id = o.id ORDER BY value_date DESC, id DESC LIMIT 1
	)`

func scanFinancialObject(scanner interface{ Scan(...any) error }) (FinancialObject, error) {
	var item FinancialObject
	var valueID, valueCents sql.NullInt64
	var valueDate sql.NullString
	err := scanner.Scan(&item.ID, &item.ObjectType, &item.Name, &item.Description, &valueID, &valueDate, &valueCents)
	if valueID.Valid {
		item.CurrentValue = &ObjectValue{ID: valueID.Int64, FinancialObjectID: item.ID, ValueDate: valueDate.String, ValueCents: valueCents.Int64}
	}
	return item, coreStoreError(err)
}

func (s *Core) FinancialObjects(ctx context.Context, objectType string) ([]FinancialObject, error) {
	if objectType != "" && !validFinancialObjectType(objectType) {
		return nil, ErrCoreInvalid
	}
	rows, err := s.db.QueryContext(ctx, financialObjectSelect+" WHERE (? = '' OR o.object_type = ?) ORDER BY o.name, o.id", objectType, objectType)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []FinancialObject{}
	for rows.Next() {
		item, err := scanFinancialObject(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Core) FinancialObject(ctx context.Context, id int64) (FinancialObject, error) {
	return scanFinancialObject(s.db.QueryRowContext(ctx, financialObjectSelect+" WHERE o.id = ?", id))
}

func (s *Core) CreateFinancialObject(ctx context.Context, input SaveFinancialObject) (FinancialObject, error) {
	input, err := normalizeFinancialObject(input)
	if err != nil {
		return FinancialObject{}, err
	}
	result, err := s.db.ExecContext(ctx, "INSERT INTO financial_objects (object_type, name, description) VALUES (?, ?, ?)", input.ObjectType, input.Name, input.Description)
	if err != nil {
		return FinancialObject{}, coreStoreError(err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return FinancialObject{}, err
	}
	return s.FinancialObject(ctx, id)
}

func (s *Core) UpdateFinancialObject(ctx context.Context, id int64, input SaveFinancialObject) (FinancialObject, error) {
	input, err := normalizeFinancialObject(input)
	if err != nil {
		return FinancialObject{}, err
	}
	result, err := s.db.ExecContext(ctx, "UPDATE financial_objects SET object_type = ?, name = ?, description = ? WHERE id = ?", input.ObjectType, input.Name, input.Description, id)
	if err := coreMutationError(result, err); err != nil {
		return FinancialObject{}, err
	}
	return s.FinancialObject(ctx, id)
}

// DeleteFinancialObject verwijdert alleen losse objecten. Waarden, relaties
// en documentkoppelingen zijn historische gegevens en blokkeren verwijdering.
func (s *Core) DeleteFinancialObject(ctx context.Context, id int64) error {
	if _, err := s.FinancialObject(ctx, id); err != nil {
		return err
	}
	var linked int
	if err := s.db.QueryRowContext(ctx, `SELECT (
		(SELECT count(*) FROM object_values WHERE financial_object_id = ?) +
		(SELECT count(*) FROM object_relations WHERE source_object_id = ? OR target_object_id = ?) +
		(SELECT count(*) FROM document_financial_objects WHERE financial_object_id = ?))`, id, id, id, id).Scan(&linked); err != nil {
		return coreStoreError(err)
	}
	if linked > 0 {
		return ErrCoreConflict
	}
	result, err := s.db.ExecContext(ctx, "DELETE FROM financial_objects WHERE id = ?", id)
	return coreMutationError(result, err)
}

func (s *Core) ObjectValues(ctx context.Context, objectID int64) ([]ObjectValue, error) {
	if _, err := s.FinancialObject(ctx, objectID); err != nil {
		return nil, err
	}
	rows, err := s.db.QueryContext(ctx, "SELECT id, financial_object_id, value_date, value_cents FROM object_values WHERE financial_object_id = ? ORDER BY value_date DESC, id DESC", objectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []ObjectValue{}
	for rows.Next() {
		var item ObjectValue
		if err := rows.Scan(&item.ID, &item.FinancialObjectID, &item.ValueDate, &item.ValueCents); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Core) CreateObjectValue(ctx context.Context, objectID int64, input SaveObjectValue) (ObjectValue, error) {
	date, err := time.Parse("2006-01-02", input.ValueDate)
	if err != nil || date.Format("2006-01-02") != input.ValueDate || input.ValueCents == nil {
		return ObjectValue{}, fmt.Errorf("%w: vul een geldige waardedatum en een bedrag in centen in", ErrCoreInvalid)
	}
	if _, err := s.FinancialObject(ctx, objectID); err != nil {
		return ObjectValue{}, err
	}
	result, err := s.db.ExecContext(ctx, "INSERT INTO object_values (financial_object_id, value_date, value_cents) VALUES (?, ?, ?)", objectID, input.ValueDate, *input.ValueCents)
	if err != nil {
		return ObjectValue{}, coreStoreError(err)
	}
	id, err := result.LastInsertId()
	return ObjectValue{ID: id, FinancialObjectID: objectID, ValueDate: input.ValueDate, ValueCents: *input.ValueCents}, err
}

func (s *Core) DeleteObjectValue(ctx context.Context, objectID, valueID int64) error {
	result, err := s.db.ExecContext(ctx, "DELETE FROM object_values WHERE id = ? AND financial_object_id = ?", valueID, objectID)
	return coreMutationError(result, err)
}

func (s *Core) ObjectRelations(ctx context.Context, objectID int64) ([]ObjectRelation, error) {
	if _, err := s.FinancialObject(ctx, objectID); err != nil {
		return nil, err
	}
	rows, err := s.db.QueryContext(ctx, `SELECT source_object_id, target_object_id, relation_type FROM object_relations
		WHERE source_object_id = ? OR target_object_id = ? ORDER BY source_object_id, target_object_id, relation_type`, objectID, objectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []ObjectRelation{}
	for rows.Next() {
		var item ObjectRelation
		if err := rows.Scan(&item.SourceObjectID, &item.TargetObjectID, &item.RelationType); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func normalizeObjectRelation(input ObjectRelation) (ObjectRelation, error) {
	input.RelationType = strings.TrimSpace(input.RelationType)
	if input.SourceObjectID <= 0 || input.TargetObjectID <= 0 || input.SourceObjectID == input.TargetObjectID || input.RelationType == "" {
		return input, fmt.Errorf("%w: kies twee verschillende objecten en vul een relatietype in", ErrCoreInvalid)
	}
	return input, nil
}

func (s *Core) CreateObjectRelation(ctx context.Context, input ObjectRelation) (ObjectRelation, error) {
	input, err := normalizeObjectRelation(input)
	if err != nil {
		return ObjectRelation{}, err
	}
	for _, id := range []int64{input.SourceObjectID, input.TargetObjectID} {
		if _, err := s.FinancialObject(ctx, id); err != nil {
			return ObjectRelation{}, err
		}
	}
	_, err = s.db.ExecContext(ctx, "INSERT INTO object_relations (source_object_id, target_object_id, relation_type) VALUES (?, ?, ?)", input.SourceObjectID, input.TargetObjectID, input.RelationType)
	return input, coreStoreError(err)
}

func (s *Core) DeleteObjectRelation(ctx context.Context, input ObjectRelation) error {
	input, err := normalizeObjectRelation(input)
	if err != nil {
		return err
	}
	result, err := s.db.ExecContext(ctx, "DELETE FROM object_relations WHERE source_object_id = ? AND target_object_id = ? AND relation_type = ?", input.SourceObjectID, input.TargetObjectID, input.RelationType)
	return coreMutationError(result, err)
}

func coreStoreError(err error) error {
	if errors.Is(err, sql.ErrNoRows) {
		return ErrCoreNotFound
	}
	var sqliteError *sqlite.Error
	if errors.As(err, &sqliteError) && sqliteError.Code()&0xff == 19 {
		return ErrCoreConflict
	}
	return err
}

func coreMutationError(result sql.Result, err error) error {
	if err != nil {
		return coreStoreError(err)
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count == 0 {
		return ErrCoreNotFound
	}
	return nil
}
