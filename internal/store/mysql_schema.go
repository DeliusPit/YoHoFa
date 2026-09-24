package store

var mysqlMigrations = []struct {
	version    string
	statements []string
}{
	{
		version: "001_canonical_schema",
		statements: []string{
			`create table if not exists households (
				id bigint not null auto_increment primary key,
				name varchar(255) not null,
				created_at datetime(6) not null default current_timestamp(6)
			) engine=InnoDB default charset=utf8mb4 collate=utf8mb4_0900_as_cs`,
			`create table if not exists accounts (
				id bigint not null auto_increment primary key,
				household_id bigint not null default 1,
				name varchar(255) not null,
				iban varchar(64),
				account_type varchar(16) not null default 'checking',
				owner_name varchar(255),
				bank_profile varchar(64) not null default 'generic',
				currency char(3) not null default 'EUR',
				opening_balance_cents bigint not null default 0,
				sort_order int not null default 100,
				created_at datetime(6) not null default current_timestamp(6),
				iban_unique varchar(64) generated always as (nullif(iban, '')) stored,
				constraint accounts_type_check check (account_type in ('checking', 'savings')),
				constraint accounts_household_fk foreign key (household_id) references households(id) on delete cascade,
				unique key accounts_iban_unique (iban_unique)
			) engine=InnoDB default charset=utf8mb4 collate=utf8mb4_0900_as_cs`,
			`create table if not exists main_categories (
				id bigint not null auto_increment primary key,
				name varchar(255) not null,
				sort_order int not null default 100,
				created_at datetime(6) not null default current_timestamp(6),
				unique key main_categories_name_unique (name)
			) engine=InnoDB default charset=utf8mb4 collate=utf8mb4_0900_as_cs`,
			`create table if not exists categories (
				id bigint not null auto_increment primary key,
				household_id bigint not null default 1,
				main_category_id bigint,
				name varchar(255) not null,
				kind varchar(16) not null,
				purpose varchar(16) not null default 'daily',
				sort_order int not null default 100,
				created_at datetime(6) not null default current_timestamp(6),
				constraint categories_kind_check check (kind in ('income', 'expense', 'transfer')),
				constraint categories_purpose_check check (purpose in ('daily', 'reservation', 'investment')),
				constraint categories_household_fk foreign key (household_id) references households(id) on delete cascade,
				constraint categories_main_fk foreign key (main_category_id) references main_categories(id) on delete cascade,
				unique key categories_main_name_unique (main_category_id, name),
				key categories_main_purpose_idx (main_category_id, purpose, sort_order)
			) engine=InnoDB default charset=utf8mb4 collate=utf8mb4_0900_as_cs`,
			`create table if not exists transactions (
				id bigint not null auto_increment primary key,
				account_id bigint not null,
				category_id bigint,
				booked_on date not null,
				description varchar(1024) not null,
				counterparty varchar(255),
				counterparty_iban varchar(64),
				amount_cents bigint not null,
				external_reference varchar(255),
				import_fingerprint varchar(128),
				hidden boolean not null default false,
				created_at datetime(6) not null default current_timestamp(6),
				fingerprint_unique varchar(128) generated always as (nullif(import_fingerprint, '')) stored,
				constraint transactions_account_fk foreign key (account_id) references accounts(id) on delete cascade,
				constraint transactions_category_fk foreign key (category_id) references categories(id) on delete set null,
				unique key transactions_import_fingerprint_unique (fingerprint_unique),
				key transactions_account_booked_on_idx (account_id, booked_on desc),
				key transactions_category_idx (category_id)
			) engine=InnoDB default charset=utf8mb4 collate=utf8mb4_0900_as_cs`,
			`create table if not exists documents (
				id bigint not null auto_increment primary key,
				original_filename varchar(512) not null,
				mime_type varchar(255) not null,
				size_bytes bigint not null,
				sha256 char(64) not null,
				object_key varchar(700) not null,
				document_date date,
				description varchar(2048),
				created_at datetime(6) not null default current_timestamp(6),
				constraint documents_size_check check (size_bytes >= 0),
				unique key documents_object_key_unique (object_key),
				key documents_sha256_idx (sha256)
			) engine=InnoDB default charset=utf8mb4 collate=utf8mb4_0900_as_cs`,
			`create table if not exists assets (
				id bigint not null auto_increment primary key,
				household_id bigint not null default 1,
				asset_type varchar(32) not null,
				name varchar(255) not null,
				description varchar(2048),
				status varchar(16) not null default 'active',
				acquired_at date,
				disposed_at date,
				purchase_value bigint not null default 0,
				current_value bigint not null default 0,
				currency char(3) not null default 'EUR',
				created_at datetime(6) not null default current_timestamp(6),
				updated_at datetime(6) not null default current_timestamp(6),
				constraint assets_type_check check (asset_type in ('real_estate','bank_account','investment_account','vehicle','equipment','valuable','cash','other')),
				constraint assets_status_check check (status in ('planned','active','disposed')),
				constraint assets_values_check check (purchase_value >= 0 and current_value >= 0),
				constraint assets_household_fk foreign key (household_id) references households(id) on delete cascade,
				key assets_household_type_status_idx (household_id, asset_type, status, name)
			) engine=InnoDB default charset=utf8mb4 collate=utf8mb4_0900_as_cs`,
			`create table if not exists liabilities (
				id bigint not null auto_increment primary key,
				household_id bigint not null default 1,
				liability_type varchar(24) not null,
				name varchar(255) not null,
				description varchar(2048), creditor varchar(255),
				start_date date, end_date date,
				original_amount bigint not null default 0,
				current_balance bigint not null default 0,
				currency char(3) not null default 'EUR', status varchar(16) not null default 'active',
				created_at datetime(6) not null default current_timestamp(6), updated_at datetime(6) not null default current_timestamp(6),
				constraint liabilities_type_check check (liability_type in ('mortgage','loan','tax_debt','credit','other')),
				constraint liabilities_status_check check (status in ('planned','active','repaid','cancelled')),
				constraint liabilities_amount_check check (original_amount >= 0 and current_balance >= 0),
				constraint liabilities_dates_check check (end_date is null or start_date is null or end_date >= start_date),
				constraint liabilities_household_fk foreign key (household_id) references households(id) on delete cascade,
				key liabilities_household_type_status_idx (household_id, liability_type, status, name)
			) engine=InnoDB default charset=utf8mb4 collate=utf8mb4_0900_as_cs`,
			`create table if not exists cases (
				id bigint not null auto_increment primary key, household_id bigint not null,
				case_type varchar(128) not null, title varchar(255) not null, description varchar(2048),
				start_date date, end_date date, status varchar(16) not null default 'active',
				created_at datetime(6) not null default current_timestamp(6), updated_at datetime(6) not null default current_timestamp(6),
				constraint cases_type_check check (trim(case_type) <> ''), constraint cases_title_check check (trim(title) <> ''),
				constraint cases_status_check check (status in ('planned','active','completed','archived')),
				constraint cases_dates_check check (end_date is null or start_date is null or end_date >= start_date),
				constraint cases_household_fk foreign key (household_id) references households(id) on delete cascade,
				key cases_household_type_status_title_idx (household_id, case_type, status, title)
			) engine=InnoDB default charset=utf8mb4 collate=utf8mb4_0900_as_cs`,
			`create table if not exists asset_valuations (
				id bigint not null auto_increment primary key, asset_id bigint not null, valuation_date date not null,
				value bigint not null, valuation_type varchar(16) not null, source varchar(512), notes varchar(2048),
				created_at datetime(6) not null default current_timestamp(6),
				constraint valuations_value_check check (value >= 0),
				constraint valuations_type_check check (valuation_type in ('purchase','estimate','appraisal','tax','market','other')),
				constraint valuations_asset_fk foreign key (asset_id) references assets(id) on delete cascade,
				key asset_valuations_asset_date_idx (asset_id, valuation_date desc, id desc)
			) engine=InnoDB default charset=utf8mb4 collate=utf8mb4_0900_as_cs`,
			`create table if not exists entity_relations (
				id bigint not null auto_increment primary key,
				source_type varchar(16) not null, source_id bigint not null, target_type varchar(16) not null, target_id bigint not null,
				relation_type varchar(16) not null, valid_from date, valid_to date, notes varchar(2048),
				created_at datetime(6) not null default current_timestamp(6),
				valid_from_key date generated always as (coalesce(valid_from, date '1000-01-01')) stored,
				valid_to_key date generated always as (coalesce(valid_to, date '9999-12-31')) stored,
				constraint relations_self_check check (source_type <> target_type or source_id <> target_id),
				constraint relations_dates_check check (valid_to is null or valid_from is null or valid_to >= valid_from),
				constraint relations_type_check check (relation_type in ('relates_to','evidence_for','financed_by')),
				unique key entity_relations_unique_idx (source_type, source_id, target_type, target_id, relation_type, valid_from_key, valid_to_key),
				key entity_relations_source_idx (source_type, source_id, relation_type),
				key entity_relations_target_idx (target_type, target_id, relation_type)
			) engine=InnoDB default charset=utf8mb4 collate=utf8mb4_0900_as_cs`,
			`create table if not exists domain_events (
				id bigint not null auto_increment primary key,
				entity_type varchar(16) not null, entity_id bigint not null, event_type varchar(64) not null,
				event_date date not null, description varchar(1024) not null,
				related_entity_type varchar(16), related_entity_id bigint,
				created_at datetime(6) not null default current_timestamp(6),
				creation_key tinyint generated always as (case when event_type in ('asset_created','liability_created','case_created') then 1 else null end) stored,
				constraint events_owner_check check (entity_type in ('asset','liability','case') and entity_id > 0),
				constraint events_required_check check (trim(event_type) <> '' and trim(description) <> ''),
				constraint events_related_pair_check check ((related_entity_type is null) = (related_entity_id is null)),
				constraint events_related_type_check check (related_entity_type is null or related_entity_type in ('document','transaction','asset','liability','case')),
				constraint events_related_id_check check (related_entity_id is null or related_entity_id > 0),
				constraint events_self_check check (related_entity_type is null or entity_type <> related_entity_type or entity_id <> related_entity_id),
				unique key domain_events_creation_unique_idx (entity_type, entity_id, creation_key),
				key domain_events_timeline_idx (entity_type, entity_id, event_date desc, id desc)
			) engine=InnoDB default charset=utf8mb4 collate=utf8mb4_0900_as_cs`,
		},
	},
	{
		version: "002_main_category_cases",
		statements: []string{
			`alter table cases add column main_category_id bigint null after household_id`,
			`alter table cases add constraint cases_main_category_fk foreign key (main_category_id) references main_categories(id) on delete cascade`,
			`alter table cases add unique key cases_main_category_unique_idx (main_category_id)`,
		},
	},
	{
		version: "003_case_contract_fields",
		statements: []string{
			`alter table cases add column term_months int null after end_date`,
			`alter table cases add constraint cases_term_months_check check (term_months is null or term_months > 0)`,
		},
	},
	{
		version: "004_reservations",
		statements: []string{
			`alter table domain_events drop check if exists events_owner_check`,
			`alter table domain_events add constraint events_owner_check check (entity_type in ('asset','liability','case','reservation') and entity_id > 0)`,
			`alter table domain_events drop check if exists events_related_type_check`,
			`alter table domain_events add constraint events_related_type_check check (related_entity_type is null or related_entity_type in ('document','transaction','asset','liability','case','reservation'))`,
			`create table if not exists reservations (
				id bigint not null auto_increment primary key, household_id bigint not null,
				reserve_type varchar(32) not null, name varchar(255) not null, description varchar(2048), due_date date,
				amount bigint not null default 0, currency char(3) not null default 'EUR',
				created_at datetime(6) not null default current_timestamp(6), updated_at datetime(6) not null default current_timestamp(6),
				constraint reservations_type_check check (reserve_type in ('mortgage','loan','tax_debt','credit','other')),
				constraint reservations_name_check check (trim(name) <> ''),
				constraint reservations_amount_check check (amount >= 0),
				constraint reservations_household_fk foreign key (household_id) references households(id) on delete cascade,
				key reservations_household_type_due_idx (household_id, reserve_type, due_date, name)
			) engine=InnoDB default charset=utf8mb4 collate=utf8mb4_0900_as_cs`,
		},
	},
	{
		version: "005_reservation_creation_event",
		statements: []string{
			`delete duplicate_event from domain_events duplicate_event
				join domain_events earlier_event on earlier_event.entity_type = duplicate_event.entity_type
					and earlier_event.entity_id = duplicate_event.entity_id
					and earlier_event.event_type = duplicate_event.event_type
					and earlier_event.id < duplicate_event.id
				where duplicate_event.event_type in ('asset_created','liability_created','case_created','reservation_created')`,
			`alter table domain_events
				drop index domain_events_creation_unique_idx,
				drop column creation_key,
				add column creation_key tinyint generated always as (case when event_type in ('asset_created','liability_created','case_created','reservation_created') then 1 else null end) stored,
				add unique key domain_events_creation_unique_idx (entity_type, entity_id, creation_key)`,
		},
	},
	{
		version: "006_document_translations",
		statements: []string{
			`alter table documents add column translation_text text null after description`,
			`alter table documents add column translation_status varchar(16) not null default 'not_requested' after translation_text`,
			`alter table documents add constraint documents_translation_status_check check (translation_status in ('not_requested','pending','completed','unsupported','failed'))`,
		},
	},
	{
		version: "007_document_analyses_parties",
		statements: []string{
			`create table if not exists parties (
				id bigint primary key auto_increment,
				household_id bigint not null default 1,
				party_type varchar(32) not null default 'organization',
				name varchar(255) not null,
				notes text,
				created_at timestamp(6) not null default current_timestamp(6),
				updated_at timestamp(6) not null default current_timestamp(6) on update current_timestamp(6),
				constraint parties_household_fk foreign key (household_id) references households(id) on delete cascade,
				constraint parties_type_check check (party_type in ('person','organization','bank','government','insurer','supplier','other')),
				constraint parties_name_check check (trim(name) <> ''),
				unique key parties_household_id_unique (household_id, id),
				key parties_household_name_idx (household_id, name)
			)`,
			`create table if not exists party_identifiers (
				id bigint primary key auto_increment,
				household_id bigint not null default 1,
				party_id bigint not null,
				identifier_type varchar(32) not null,
				identifier_value varchar(255) not null,
				normalized_value varchar(255) not null,
				created_at timestamp(6) not null default current_timestamp(6),
				constraint party_identifiers_household_fk foreign key (household_id) references households(id) on delete cascade,
				constraint party_identifiers_party_fk foreign key (party_id) references parties(id) on delete cascade,
				constraint party_identifiers_type_check check (identifier_type in ('iban','bic','creditor_id','vat_id','tax_id','customer_number','email','phone','other')),
				constraint party_identifiers_value_check check (trim(identifier_value) <> '' and trim(normalized_value) <> ''),
				unique key party_identifiers_unique (household_id, identifier_type, normalized_value),
				key party_identifiers_party_idx (party_id)
			)`,
			`alter table liabilities add column creditor_party_id bigint null after creditor`,
			`alter table liabilities add constraint liabilities_creditor_party_fk foreign key (creditor_party_id) references parties(id) on delete set null`,
			`create index liabilities_creditor_party_idx on liabilities(creditor_party_id)`,
			`insert into parties(household_id, party_type, name)
				select distinct liabilities.household_id, 'organization', trim(liabilities.creditor)
				from liabilities
				where trim(coalesce(liabilities.creditor, '')) <> ''
				and not exists (select 1 from parties where parties.household_id = liabilities.household_id and lower(parties.name) = lower(trim(liabilities.creditor)))`,
			`update liabilities join parties on parties.household_id = liabilities.household_id and lower(parties.name) = lower(trim(liabilities.creditor))
				set liabilities.creditor_party_id = parties.id
				where liabilities.creditor_party_id is null and trim(coalesce(liabilities.creditor, '')) <> ''`,
			`create table if not exists document_analyses (
				id bigint primary key auto_increment,
				document_id bigint not null,
				analysis_type varchar(16) not null,
				status varchar(16) not null default 'pending',
				language varchar(16),
				model varchar(255),
				content_text text,
				content_json json,
				error_message text,
				created_at timestamp(6) not null default current_timestamp(6),
				updated_at timestamp(6) not null default current_timestamp(6) on update current_timestamp(6),
				constraint document_analyses_document_fk foreign key (document_id) references documents(id) on delete cascade,
				constraint document_analyses_type_check check (analysis_type in ('translation','analysis')),
				constraint document_analyses_status_check check (status in ('not_requested','pending','completed','unsupported','failed')),
				unique key document_analyses_unique (document_id, analysis_type)
			)`,
			`insert into document_analyses(document_id, analysis_type, status, language, content_text, created_at, updated_at)
				select id, 'translation', translation_status, 'nl', nullif(translation_text, ''), created_at, current_timestamp(6)
				from documents
				where translation_status <> 'not_requested' or trim(coalesce(translation_text, '')) <> ''
				on duplicate key update status=values(status), language=coalesce(document_analyses.language, values(language)), content_text=coalesce(nullif(document_analyses.content_text, ''), values(content_text)), updated_at=current_timestamp(6)`,
		},
	},
	{
		version: "008_transaction_counterparties",
		statements: []string{
			`alter table transactions add column counterparty_party_id bigint null after counterparty_iban`,
			`alter table transactions add constraint transactions_counterparty_party_fk foreign key (counterparty_party_id) references parties(id) on delete set null`,
			`alter table transactions add column counterparty_name_raw varchar(255) null after counterparty_party_id`,
			`alter table transactions add column counterparty_iban_raw varchar(64) null after counterparty_name_raw`,
			`update transactions set counterparty_name_raw = counterparty where counterparty_name_raw is null and trim(coalesce(counterparty, '')) <> ''`,
			`update transactions set counterparty_iban_raw = counterparty_iban where counterparty_iban_raw is null and trim(coalesce(counterparty_iban, '')) <> ''`,
			`insert into parties(household_id, party_type, name)
				select distinct accounts.household_id, 'organization', trim(transactions.counterparty_name_raw)
				from transactions join accounts on accounts.id = transactions.account_id
				where trim(coalesce(transactions.counterparty_name_raw, '')) <> ''
				and not exists (select 1 from parties where parties.household_id = accounts.household_id and lower(parties.name) = lower(trim(transactions.counterparty_name_raw)))`,
			`update transactions join accounts on accounts.id = transactions.account_id join parties on parties.household_id = accounts.household_id and lower(parties.name) = lower(trim(transactions.counterparty_name_raw))
				set transactions.counterparty_party_id = parties.id
				where transactions.counterparty_party_id is null and trim(coalesce(transactions.counterparty_name_raw, '')) <> ''`,
			`insert into party_identifiers(household_id, party_id, identifier_type, identifier_value, normalized_value)
				select distinct accounts.household_id, transactions.counterparty_party_id, 'iban', trim(transactions.counterparty_iban_raw), upper(replace(trim(transactions.counterparty_iban_raw), ' ', ''))
				from transactions join accounts on accounts.id = transactions.account_id
				where transactions.counterparty_party_id is not null and trim(coalesce(transactions.counterparty_iban_raw, '')) <> ''
				on duplicate key update id = id`,
			`create index transactions_counterparty_party_idx on transactions(counterparty_party_id, booked_on)`,
		},
	},
}
