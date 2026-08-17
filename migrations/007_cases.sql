begin;
select pg_advisory_xact_lock(684523817);

create table if not exists cases (
    id bigserial primary key,
    household_id bigint not null references households(id) on delete cascade,
    case_type text not null check (btrim(case_type) <> ''),
    title text not null check (btrim(title) <> ''),
    description text,
    start_date date,
    end_date date,
    status text not null default 'active' check (status in ('planned', 'active', 'completed', 'archived')),
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now(),
    check (end_date is null or start_date is null or end_date >= start_date)
);

create index if not exists cases_household_type_status_title_idx on cases(household_id, case_type, status, title);

insert into schema_migrations (version) values ('007_cases') on conflict (version) do nothing;

commit;
