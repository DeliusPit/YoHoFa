create table households (
    id bigserial primary key,
    name text not null,
    created_at timestamptz not null default now()
);

create table household_members (
    id bigserial primary key,
    household_id bigint not null references households(id) on delete cascade,
    name text not null,
    email text,
    created_at timestamptz not null default now()
);

create table accounts (
    id bigserial primary key,
    household_id bigint not null references households(id) on delete cascade,
    name text not null,
    iban text,
    account_type text not null default 'checking',
    owner_name text,
    bank_profile text not null default 'generic',
    currency char(3) not null default 'EUR',
    opening_balance_cents bigint not null default 0,
    sort_order integer not null default 100,
    created_at timestamptz not null default now()
);

create table main_categories (
    id bigserial primary key,
    name text not null unique,
    sort_order integer not null default 100,
    created_at timestamptz not null default now()
);

insert into main_categories (name, sort_order) values
    ('Inkomsten', 10),
    ('Wonen', 20),
    ('Huishouden', 30),
    ('Medische kosten', 40),
    ('Verzekeringen', 50),
    ('Telecom', 60),
    ('Vervoer', 70),
    ('Educatie', 80),
    ('Kleding', 90),
    ('Vrije tijd', 100),
    ('Overige uitgaven', 110);

create table categories (
    id bigserial primary key,
    household_id bigint not null references households(id) on delete cascade,
    main_category_id bigint references main_categories(id) on delete set null,
    name text not null,
    kind text not null check (kind in ('income', 'expense', 'transfer')),
    purpose text not null default 'daily' check (purpose in ('daily', 'reservation', 'investment')),
    sort_order integer not null default 100,
    created_at timestamptz not null default now(),
    unique (household_id, name)
);

create index categories_household_purpose_idx on categories(household_id, purpose, sort_order);

create table transactions (
    id bigserial primary key,
    account_id bigint not null references accounts(id) on delete cascade,
    category_id bigint references categories(id) on delete set null,
    booked_on date not null,
    description text not null,
    counterparty text,
    amount_cents bigint not null,
    external_reference text,
    created_at timestamptz not null default now()
);

create index transactions_account_booked_on_idx on transactions(account_id, booked_on desc);
create index transactions_category_idx on transactions(category_id);

create table budgets (
    id bigserial primary key,
    household_id bigint not null references households(id) on delete cascade,
    category_id bigint not null references categories(id) on delete cascade,
    period_month date not null,
    amount_cents bigint not null,
    created_at timestamptz not null default now(),
    unique (household_id, category_id, period_month)
);
