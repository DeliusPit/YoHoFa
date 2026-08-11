create table if not exists households (
    id bigserial primary key,
    name text not null,
    created_at timestamptz not null default now()
);

insert into households (id, name) values (1, 'Huishouden') on conflict (id) do nothing;

create table if not exists accounts (
    id bigserial primary key,
    household_id bigint not null default 1 references households(id) on delete cascade,
    name text not null,
    iban text,
    account_type text not null default 'checking' check (account_type in ('checking', 'savings')),
    owner_name text,
    bank_profile text not null default 'generic',
    currency char(3) not null default 'EUR',
    opening_balance_cents bigint not null default 0,
    sort_order integer not null default 100,
    created_at timestamptz not null default now()
);

create unique index if not exists accounts_iban_unique on accounts(iban) where iban is not null and iban != '';

create table if not exists main_categories (
    id bigserial primary key,
    name text not null unique,
    sort_order integer not null default 100,
    created_at timestamptz not null default now()
);

create table if not exists categories (
    id bigserial primary key,
    household_id bigint not null default 1 references households(id) on delete cascade,
    main_category_id bigint references main_categories(id) on delete cascade,
    name text not null,
    kind text not null check (kind in ('income', 'expense', 'transfer')),
    purpose text not null default 'daily' check (purpose in ('daily', 'reservation', 'investment')),
    sort_order integer not null default 100,
    created_at timestamptz not null default now(),
    unique (main_category_id, name)
);

create index if not exists categories_main_purpose_idx on categories(main_category_id, purpose, sort_order);
create unique index if not exists categories_main_name_unique on categories(main_category_id, name);

create table if not exists transactions (
    id bigserial primary key,
    account_id bigint not null references accounts(id) on delete cascade,
    category_id bigint references categories(id) on delete set null,
    booked_on date not null,
    description text not null,
    counterparty text,
    counterparty_iban text,
    amount_cents bigint not null,
    external_reference text,
    import_fingerprint text,
    hidden boolean not null default false,
    created_at timestamptz not null default now()
);

create index if not exists transactions_account_booked_on_idx on transactions(account_id, booked_on desc);
create index if not exists transactions_category_idx on transactions(category_id);
create unique index if not exists transactions_import_fingerprint_unique on transactions(import_fingerprint) where import_fingerprint is not null and import_fingerprint != '';

insert into main_categories (id, name, sort_order) values
    (1, 'Inkomsten', 10),
    (2, 'Wonen', 20),
    (3, 'Huishouden', 30),
    (4, 'Medische kosten', 40),
    (5, 'Verzekeringen', 50),
    (6, 'Telecom', 60),
    (7, 'Vervoer', 70),
    (8, 'Educatie', 80),
    (9, 'Kleding', 90),
    (10, 'Vrije tijd', 100),
    (11, 'Overige uitgaven', 110)
on conflict (id) do nothing;

insert into accounts (id, household_id, name, iban, account_type, owner_name, bank_profile, sort_order) values
    (1, 1, 'Triodos betaalrekening', 'NL18TRIO0254363105', 'checking', 'Huishouden', 'triodos', 10)
on conflict (id) do nothing;

insert into categories (id, household_id, main_category_id, name, kind, purpose, sort_order) values
    (1, 1, 3, 'Boodschappen', 'expense', 'daily', 10),
    (2, 1, 2, 'Wonen', 'expense', 'daily', 20),
    (3, 1, 10, 'Vakantie', 'expense', 'reservation', 30),
    (4, 1, 2, 'Onderhoud huis', 'expense', 'reservation', 40),
    (5, 1, 2, 'Verduurzaming woning', 'expense', 'investment', 50),
    (6, 1, 2, 'Verbouwing', 'expense', 'investment', 60),
    (7, 1, 1, 'Salaris', 'income', 'daily', 70)
on conflict (id) do nothing;

select setval(pg_get_serial_sequence('households', 'id'), coalesce((select max(id) from households), 1), true);
select setval(pg_get_serial_sequence('accounts', 'id'), coalesce((select max(id) from accounts), 1), true);
select setval(pg_get_serial_sequence('main_categories', 'id'), coalesce((select max(id) from main_categories), 1), true);
select setval(pg_get_serial_sequence('categories', 'id'), coalesce((select max(id) from categories), 1), true);
select setval(pg_get_serial_sequence('transactions', 'id'), coalesce((select max(id) from transactions), 1), true);
