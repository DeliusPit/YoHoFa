begin;
select pg_advisory_xact_lock(684523817);

create table if not exists liabilities (
    id bigserial primary key,
    household_id bigint not null default 1 references households(id) on delete cascade,
    liability_type text not null check (liability_type in ('mortgage', 'loan', 'tax_debt', 'credit', 'other')),
    name text not null,
    description text,
    creditor text,
    start_date date,
    end_date date,
    original_amount bigint not null default 0 check (original_amount >= 0),
    current_balance bigint not null default 0 check (current_balance >= 0),
    currency char(3) not null default 'EUR',
    status text not null default 'active' check (status in ('planned', 'active', 'repaid', 'cancelled')),
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now(),
    check (end_date is null or start_date is null or end_date >= start_date)
);

create index if not exists liabilities_household_type_status_idx on liabilities(household_id, liability_type, status, name);

alter table entity_relations drop constraint if exists entity_relations_relation_type_check;
alter table entity_relations add constraint entity_relations_relation_type_check
    check (relation_type in ('relates_to', 'evidence_for', 'financed_by'));

insert into schema_migrations (version) values ('006_liabilities') on conflict (version) do nothing;

commit;
