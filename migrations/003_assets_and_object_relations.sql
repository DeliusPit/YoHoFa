create table if not exists assets (
    id bigserial primary key,
    household_id bigint not null default 1 references households(id) on delete cascade,
    asset_type text not null check (asset_type in ('real_estate', 'bank_account', 'investment_account', 'vehicle', 'equipment', 'valuable', 'cash', 'other')),
    name text not null,
    description text,
    status text not null default 'active' check (status in ('planned', 'active', 'disposed')),
    acquired_at date,
    disposed_at date,
    purchase_value bigint not null default 0 check (purchase_value >= 0),
    current_value bigint not null default 0 check (current_value >= 0),
    currency char(3) not null default 'EUR',
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now()
);

create index if not exists assets_household_type_status_idx on assets(household_id, asset_type, status, name);

create table if not exists object_relations (
    object_type text not null,
    object_id bigint not null,
    related_object_type text not null,
    related_object_id bigint not null,
    relation_type text not null default 'related',
    created_at timestamptz not null default now(),
    primary key (object_type, object_id, related_object_type, related_object_id, relation_type),
    check (object_type <> related_object_type or object_id <> related_object_id)
);

create index if not exists object_relations_related_idx on object_relations(related_object_type, related_object_id, relation_type);
