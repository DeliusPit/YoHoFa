package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	drivermysql "github.com/go-sql-driver/mysql"
)

type MySQL struct {
	db *sql.DB
}

var (
	_ Backend                  = (*MySQL)(nil)
	_ DocumentRepository       = (*MySQL)(nil)
	_ AssetRepository          = (*MySQL)(nil)
	_ LiabilityRepository      = (*MySQL)(nil)
	_ CaseRepository           = (*MySQL)(nil)
	_ EntityRelationRepository = (*MySQL)(nil)
	_ DomainEventRepository    = (*MySQL)(nil)
)

func normalizeMySQLDSN(dsn string) (string, error) {
	cfg, err := drivermysql.ParseDSN(dsn)
	if err != nil {
		return "", fmt.Errorf("invalid MySQL DSN")
	}
	if cfg.DBName == "" {
		return "", fmt.Errorf("MySQL DSN must select a database")
	}
	cfg.ParseTime = true
	cfg.Loc = time.UTC
	cfg.Collation = "utf8mb4_0900_as_cs"
	cfg.ClientFoundRows = true
	if cfg.Params == nil {
		cfg.Params = make(map[string]string)
	}
	cfg.Params["time_zone"] = "'+00:00'"
	cfg.Params["sql_mode"] = "'STRICT_TRANS_TABLES,NO_ZERO_DATE,NO_ZERO_IN_DATE,ERROR_FOR_DIVISION_BY_ZERO,NO_ENGINE_SUBSTITUTION'"
	return cfg.FormatDSN(), nil
}

func OpenMySQL(ctx context.Context, dsn string) (*MySQL, error) {
	normalized, err := normalizeMySQLDSN(dsn)
	if err != nil {
		return nil, err
	}
	db, err := sql.Open("mysql", normalized)
	if err != nil {
		return nil, fmt.Errorf("open MySQL: %w", err)
	}
	db.SetMaxOpenConns(12)
	db.SetMaxIdleConns(4)
	db.SetConnMaxLifetime(30 * time.Minute)
	db.SetConnMaxIdleTime(5 * time.Minute)
	s := &MySQL{db: db}
	if err := s.Ping(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("ping MySQL: %w", err)
	}
	if err := s.ensureSchema(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("initialize MySQL schema: %w", err)
	}
	return s, nil
}

func (s *MySQL) Close()                         { _ = s.db.Close() }
func (s *MySQL) Ping(ctx context.Context) error { return s.db.PingContext(ctx) }

func (s *MySQL) ensureSchema(ctx context.Context) error {
	conn, err := s.db.Conn(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()
	var acquired int
	if err := conn.QueryRowContext(ctx, `select get_lock('geldzaken_mysql_schema', 30)`).Scan(&acquired); err != nil {
		return err
	}
	if acquired != 1 {
		return fmt.Errorf("could not acquire schema migration lock")
	}
	defer conn.ExecContext(context.Background(), `select release_lock('geldzaken_mysql_schema')`)
	if _, err := conn.ExecContext(ctx, `create table if not exists schema_migrations (
		version varchar(128) not null primary key, applied_at datetime(6) not null default current_timestamp(6)
	) engine=InnoDB default charset=utf8mb4 collate=utf8mb4_0900_as_cs`); err != nil {
		return err
	}
	for _, migration := range mysqlMigrations {
		var exists int
		if err := conn.QueryRowContext(ctx, `select count(*) from schema_migrations where version = ?`, migration.version).Scan(&exists); err != nil {
			return err
		}
		if exists != 0 {
			continue
		}
		for _, statement := range migration.statements {
			if _, err := conn.ExecContext(ctx, statement); err != nil {
				return fmt.Errorf("migration %s: %w", migration.version, err)
			}
		}
		if _, err := conn.ExecContext(ctx, `insert into schema_migrations (version) values (?)`, migration.version); err != nil {
			return err
		}
	}
	if err := s.seedDefaults(ctx, conn); err != nil {
		return fmt.Errorf("seed defaults: %w", err)
	}
	return nil
}

func (s *MySQL) seedDefaults(ctx context.Context, conn *sql.Conn) error {
	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `insert into households (id, name) values (1, 'Huishouden') on duplicate key update id = id`); err != nil {
		return err
	}
	var count int
	if err := tx.QueryRowContext(ctx, `select count(*) from main_categories`).Scan(&count); err != nil {
		return err
	}
	if count == 0 {
		for _, v := range defaultMainCategories() {
			if _, err := tx.ExecContext(ctx, `insert into main_categories (id,name,sort_order) values (?,?,?)`, v.ID, v.Name, v.SortOrder); err != nil {
				return err
			}
		}
	}
	if _, err := tx.ExecContext(ctx, `
		insert into cases (household_id,main_category_id,case_type,title,status,created_at,updated_at)
		select (select id from households order by id limit 1),main_categories.id,'main_category',main_categories.name,'active',main_categories.created_at,main_categories.created_at
		from main_categories where not exists(select 1 from cases where cases.main_category_id=main_categories.id)
	`); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
		insert ignore into domain_events(entity_type,entity_id,event_type,event_date,description,created_at)
		select 'case',cases.id,'case_created',date(cases.created_at),'Dossier aangemaakt',cases.created_at
		from cases where cases.main_category_id is not null
	`); err != nil {
		return err
	}
	if err := tx.QueryRowContext(ctx, `select count(*) from accounts`).Scan(&count); err != nil {
		return err
	}
	if count == 0 {
		if _, err := tx.ExecContext(ctx, `insert into accounts (id,household_id,name,iban,account_type,owner_name,bank_profile,sort_order) values (1,1,'Triodos betaalrekening','NL18TRIO0254363105','checking','Huishouden','triodos',10)`); err != nil {
			return err
		}
	}
	if err := tx.QueryRowContext(ctx, `select count(*) from categories`).Scan(&count); err != nil {
		return err
	}
	if count == 0 {
		for _, v := range defaultCategories() {
			if _, err := tx.ExecContext(ctx, `insert into categories (id,household_id,main_category_id,name,kind,purpose,sort_order) values (?,1,?,?,?,?,?)`, v.ID, v.MainID, v.Name, v.Kind, v.Purpose, v.SortOrder); err != nil {
				return err
			}
		}
	}
	return tx.Commit()
}

func mysqlDuplicate(err error) bool {
	var mysqlErr *drivermysql.MySQLError
	return errors.As(err, &mysqlErr) && mysqlErr.Number == 1062
}
