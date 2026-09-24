package store

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
)

const ObjectTypeReservation = "reservation"

type Reservation struct {
	ID          int64  `json:"id"`
	HouseholdID int64  `json:"householdId"`
	ReserveType string `json:"reserveType"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	DueDate     string `json:"dueDate,omitempty"`
	AmountCents int64  `json:"amountCents"`
	Currency    string `json:"currency"`
	CreatedAt   string `json:"createdAt"`
	UpdatedAt   string `json:"updatedAt"`
}

type SaveReservation struct {
	ReserveType string `json:"reserveType"`
	Name        string `json:"name"`
	Description string `json:"description"`
	DueDate     string `json:"dueDate"`
	AmountCents int64  `json:"amountCents"`
	Currency    string `json:"currency"`
}

type ReservationFilter struct{ ReserveType string }

type ReservationRepository interface {
	Reservations(context.Context, ReservationFilter) ([]Reservation, error)
	Reservation(context.Context, int64) (Reservation, error)
	CreateReservation(context.Context, SaveReservation) (Reservation, error)
	UpdateReservation(context.Context, int64, SaveReservation) (Reservation, error)
	DeleteReservation(context.Context, int64) error
}

func normalizeReservationInput(input SaveReservation) (SaveReservation, any, error) {
	input.ReserveType = strings.TrimSpace(input.ReserveType)
	input.Name = strings.TrimSpace(input.Name)
	input.Description = strings.TrimSpace(input.Description)
	input.DueDate = strings.TrimSpace(input.DueDate)
	input.Currency = strings.ToUpper(strings.TrimSpace(input.Currency))
	if input.Name == "" {
		return SaveReservation{}, nil, fmt.Errorf("name is required")
	}
	if !validLiabilityType(input.ReserveType) {
		return SaveReservation{}, nil, fmt.Errorf("invalid reservation type")
	}
	if input.AmountCents < 0 {
		return SaveReservation{}, nil, fmt.Errorf("reservation amount cannot be negative")
	}
	if input.Currency == "" {
		input.Currency = "EUR"
	}
	if !validCurrency(input.Currency) {
		return SaveReservation{}, nil, fmt.Errorf("currency must contain three letters")
	}
	dueDate, err := optionalDate(input.DueDate)
	if err != nil {
		return SaveReservation{}, nil, fmt.Errorf("invalid due date")
	}
	return input, dueDate, nil
}

type reservationScanner interface{ Scan(...any) error }

func scanReservation(scanner reservationScanner, item *Reservation) error {
	return scanner.Scan(&item.ID, &item.HouseholdID, &item.ReserveType, &item.Name, &item.Description, &item.DueDate, &item.AmountCents, &item.Currency, &item.CreatedAt, &item.UpdatedAt)
}

func (s *Postgres) Reservations(ctx context.Context, filter ReservationFilter) ([]Reservation, error) {
	rows, err := s.pool.Query(ctx, `select id, household_id, reserve_type, name, coalesce(description,''), coalesce(to_char(due_date,'YYYY-MM-DD'),''), amount, currency, created_at::text, updated_at::text from reservations where ($1::text='' or reserve_type=$1) order by due_date nulls last, name, id`, strings.TrimSpace(filter.ReserveType))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []Reservation{}
	for rows.Next() {
		var item Reservation
		if err := scanReservation(rows, &item); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Postgres) Reservation(ctx context.Context, id int64) (Reservation, error) {
	var item Reservation
	err := scanReservation(s.pool.QueryRow(ctx, `select id, household_id, reserve_type, name, coalesce(description,''), coalesce(to_char(due_date,'YYYY-MM-DD'),''), amount, currency, created_at::text, updated_at::text from reservations where id=$1`, id), &item)
	if err == pgx.ErrNoRows {
		return Reservation{}, fmt.Errorf("reservation not found")
	}
	return item, err
}

func (s *Postgres) CreateReservation(ctx context.Context, input SaveReservation) (Reservation, error) {
	input, dueDate, err := normalizeReservationInput(input)
	if err != nil {
		return Reservation{}, err
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Reservation{}, err
	}
	defer tx.Rollback(ctx)
	var item Reservation
	err = scanReservation(tx.QueryRow(ctx, `insert into reservations(household_id,reserve_type,name,description,due_date,amount,currency) values ((select id from households order by id limit 1),$1,$2,$3,$4,$5,$6) returning id, household_id, reserve_type, name, coalesce(description,''), coalesce(to_char(due_date,'YYYY-MM-DD'),''), amount, currency, created_at::text, updated_at::text`, input.ReserveType, input.Name, input.Description, dueDate, input.AmountCents, input.Currency), &item)
	if err != nil {
		return Reservation{}, err
	}
	if err := appendDomainEvent(ctx, tx, AppendDomainEvent{EntityType: ObjectTypeReservation, EntityID: item.ID, EventType: "reservation_created", CurrentDate: true, Description: "Reservering aangemaakt"}); err != nil {
		return Reservation{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Reservation{}, err
	}
	return item, nil
}

func (s *Postgres) UpdateReservation(ctx context.Context, id int64, input SaveReservation) (Reservation, error) {
	input, dueDate, err := normalizeReservationInput(input)
	if err != nil {
		return Reservation{}, err
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Reservation{}, err
	}
	defer tx.Rollback(ctx)
	var item Reservation
	err = scanReservation(tx.QueryRow(ctx, `update reservations set reserve_type=$1,name=$2,description=$3,due_date=$4,amount=$5,currency=$6,updated_at=now() where id=$7 returning id, household_id, reserve_type, name, coalesce(description,''), coalesce(to_char(due_date,'YYYY-MM-DD'),''), amount, currency, created_at::text, updated_at::text`, input.ReserveType, input.Name, input.Description, dueDate, input.AmountCents, input.Currency, id), &item)
	if err == pgx.ErrNoRows {
		return Reservation{}, fmt.Errorf("reservation not found")
	} else if err != nil {
		return Reservation{}, err
	}
	if err := appendDomainEvent(ctx, tx, AppendDomainEvent{EntityType: ObjectTypeReservation, EntityID: id, EventType: "reservation_updated", CurrentDate: true, Description: "Reservering bijgewerkt"}); err != nil {
		return Reservation{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Reservation{}, err
	}
	return item, nil
}

func (s *Postgres) DeleteReservation(ctx context.Context, id int64) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var lockedID int64
	if err := tx.QueryRow(ctx, `select id from reservations where id = $1 for update`, id).Scan(&lockedID); err == pgx.ErrNoRows {
		return fmt.Errorf("reservation not found")
	} else if err != nil {
		return err
	}
	var documentCount, relationCount int
	if err := tx.QueryRow(ctx, `select count(*) from entity_relations where source_type = $1 and target_type = $2 and target_id = $3 and relation_type = $4`, ObjectTypeDocument, ObjectTypeReservation, id, RelationTypeEvidenceFor).Scan(&documentCount); err != nil {
		return err
	}
	if err := tx.QueryRow(ctx, `select count(*) from entity_relations where ((source_type = $1 and source_id = $2) or (target_type = $1 and target_id = $2)) and not (source_type = $3 and target_type = $1 and target_id = $2 and relation_type = $4)`, ObjectTypeReservation, id, ObjectTypeDocument, RelationTypeEvidenceFor).Scan(&relationCount); err != nil {
		return err
	}
	if documentCount > 0 || relationCount > 0 {
		return fmt.Errorf("reservation still has %d document links and %d object relations", documentCount, relationCount)
	}
	if err := cleanupDomainEvents(ctx, tx, ObjectTypeReservation, id); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `delete from reservations where id = $1`, id); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

const sqliteReservationSelect = `select id, household_id, reserve_type, name, coalesce(description,''), coalesce(due_date,''), amount, currency, created_at, updated_at from reservations`

func (s *SQLite) Reservations(ctx context.Context, filter ReservationFilter) ([]Reservation, error) {
	rows, err := s.db.QueryContext(ctx, sqliteReservationSelect+` where (?='' or reserve_type=?) order by due_date is null, due_date, name, id`, strings.TrimSpace(filter.ReserveType), strings.TrimSpace(filter.ReserveType))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []Reservation{}
	for rows.Next() {
		var item Reservation
		if err := scanReservation(rows, &item); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}
func (s *SQLite) Reservation(ctx context.Context, id int64) (Reservation, error) {
	var item Reservation
	err := scanReservation(s.db.QueryRowContext(ctx, sqliteReservationSelect+` where id=?`, id), &item)
	if err == sql.ErrNoRows {
		return Reservation{}, fmt.Errorf("reservation not found")
	}
	return item, err
}
func (s *SQLite) CreateReservation(ctx context.Context, input SaveReservation) (Reservation, error) {
	input, _, err := normalizeReservationInput(input)
	if err != nil {
		return Reservation{}, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Reservation{}, err
	}
	defer tx.Rollback()
	r, err := tx.ExecContext(ctx, `insert into reservations(household_id,reserve_type,name,description,due_date,amount,currency) values ((select id from households order by id limit 1),?,?,?,?,?,?)`, input.ReserveType, input.Name, input.Description, sqliteOptionalDate(input.DueDate), input.AmountCents, input.Currency)
	if err != nil {
		return Reservation{}, err
	}
	id, err := r.LastInsertId()
	if err != nil {
		return Reservation{}, err
	}
	var item Reservation
	if err := scanReservation(tx.QueryRowContext(ctx, sqliteReservationSelect+` where id=?`, id), &item); err != nil {
		return Reservation{}, err
	}
	if err := appendSQLiteDomainEvent(ctx, tx, AppendDomainEvent{EntityType: ObjectTypeReservation, EntityID: id, EventType: "reservation_created", CurrentDate: true, Description: "Reservering aangemaakt"}); err != nil {
		return Reservation{}, err
	}
	if err := tx.Commit(); err != nil {
		return Reservation{}, err
	}
	return item, nil
}
func (s *SQLite) UpdateReservation(ctx context.Context, id int64, input SaveReservation) (Reservation, error) {
	input, _, err := normalizeReservationInput(input)
	if err != nil {
		return Reservation{}, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Reservation{}, err
	}
	defer tx.Rollback()
	r, err := tx.ExecContext(ctx, `update reservations set reserve_type=?,name=?,description=?,due_date=?,amount=?,currency=?,updated_at=strftime('%Y-%m-%dT%H:%M:%fZ','now') where id=?`, input.ReserveType, input.Name, input.Description, sqliteOptionalDate(input.DueDate), input.AmountCents, input.Currency, id)
	if err != nil {
		return Reservation{}, err
	}
	if n, _ := r.RowsAffected(); n == 0 {
		return Reservation{}, fmt.Errorf("reservation not found")
	}
	var item Reservation
	if err := scanReservation(tx.QueryRowContext(ctx, sqliteReservationSelect+` where id=?`, id), &item); err != nil {
		return Reservation{}, err
	}
	if err := appendSQLiteDomainEvent(ctx, tx, AppendDomainEvent{EntityType: ObjectTypeReservation, EntityID: id, EventType: "reservation_updated", CurrentDate: true, Description: "Reservering bijgewerkt"}); err != nil {
		return Reservation{}, err
	}
	if err := tx.Commit(); err != nil {
		return Reservation{}, err
	}
	return item, nil
}
func (s *SQLite) DeleteReservation(ctx context.Context, id int64) error {
	return s.sqliteDeleteDomainObject(ctx, ObjectTypeReservation, id, "reservation")
}

const mysqlReservationColumns = `id,household_id,reserve_type,name,coalesce(description,''),coalesce(date_format(due_date,'%Y-%m-%d'),''),amount,currency,date_format(created_at,'%Y-%m-%d %H:%i:%s.%f+00'),date_format(updated_at,'%Y-%m-%d %H:%i:%s.%f+00')`

func (s *MySQL) Reservations(ctx context.Context, filter ReservationFilter) ([]Reservation, error) {
	rows, err := s.db.QueryContext(ctx, `select `+mysqlReservationColumns+` from reservations where (?='' or reserve_type=?) order by due_date is null,due_date,name,id`, strings.TrimSpace(filter.ReserveType), strings.TrimSpace(filter.ReserveType))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []Reservation{}
	for rows.Next() {
		var item Reservation
		if err := scanReservation(rows, &item); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}
func (s *MySQL) Reservation(ctx context.Context, id int64) (Reservation, error) {
	var item Reservation
	err := scanReservation(s.db.QueryRowContext(ctx, `select `+mysqlReservationColumns+` from reservations where id=?`, id), &item)
	if err == sql.ErrNoRows {
		return Reservation{}, fmt.Errorf("reservation not found")
	}
	return item, err
}
func (s *MySQL) CreateReservation(ctx context.Context, input SaveReservation) (Reservation, error) {
	input, due, err := normalizeReservationInput(input)
	if err != nil {
		return Reservation{}, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Reservation{}, err
	}
	defer tx.Rollback()
	r, err := tx.ExecContext(ctx, `insert into reservations(household_id,reserve_type,name,description,due_date,amount,currency) values ((select id from households order by id limit 1),?,?,?,?,?,?)`, input.ReserveType, input.Name, input.Description, due, input.AmountCents, input.Currency)
	if err != nil {
		return Reservation{}, err
	}
	id, err := r.LastInsertId()
	if err != nil {
		return Reservation{}, err
	}
	if err := mysqlAppendDomainEvent(ctx, tx, AppendDomainEvent{EntityType: ObjectTypeReservation, EntityID: id, EventType: "reservation_created", CurrentDate: true, Description: "Reservering aangemaakt"}); err != nil {
		return Reservation{}, err
	}
	var item Reservation
	if err := scanReservation(tx.QueryRowContext(ctx, `select `+mysqlReservationColumns+` from reservations where id=?`, id), &item); err != nil {
		return Reservation{}, err
	}
	if err := tx.Commit(); err != nil {
		return Reservation{}, err
	}
	return item, nil
}
func (s *MySQL) UpdateReservation(ctx context.Context, id int64, input SaveReservation) (Reservation, error) {
	input, due, err := normalizeReservationInput(input)
	if err != nil {
		return Reservation{}, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Reservation{}, err
	}
	defer tx.Rollback()
	r, err := tx.ExecContext(ctx, `update reservations set reserve_type=?,name=?,description=?,due_date=?,amount=?,currency=?,updated_at=utc_timestamp(6) where id=?`, input.ReserveType, input.Name, input.Description, due, input.AmountCents, input.Currency, id)
	if err != nil {
		return Reservation{}, err
	}
	if n, _ := r.RowsAffected(); n == 0 {
		return Reservation{}, fmt.Errorf("reservation not found")
	}
	if err := mysqlAppendDomainEvent(ctx, tx, AppendDomainEvent{EntityType: ObjectTypeReservation, EntityID: id, EventType: "reservation_updated", CurrentDate: true, Description: "Reservering bijgewerkt"}); err != nil {
		return Reservation{}, err
	}
	var item Reservation
	if err := scanReservation(tx.QueryRowContext(ctx, `select `+mysqlReservationColumns+` from reservations where id=?`, id), &item); err != nil {
		return Reservation{}, err
	}
	if err := tx.Commit(); err != nil {
		return Reservation{}, err
	}
	return item, nil
}
func (s *MySQL) DeleteReservation(ctx context.Context, id int64) error {
	return s.deleteDomainObject(ctx, ObjectTypeReservation, id)
}
