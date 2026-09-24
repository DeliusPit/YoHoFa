create table if not exists reservations (
    id bigserial primary key,
    household_id bigint not null default 1 references households(id) on delete cascade,
    reserve_type text not null check (reserve_type in ('mortgage', 'loan', 'tax_debt', 'credit', 'other')),
    name text not null check (btrim(name) <> ''),
    description text,
    due_date date,
    amount bigint not null default 0 check (amount >= 0),
    currency char(3) not null default 'EUR',
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now()
);

create index if not exists reservations_household_type_due_idx on reservations(household_id, reserve_type, due_date, name);

do $$
declare constraint_name text;
begin
    for constraint_name in
        select conname from pg_constraint
        where conrelid = 'domain_events'::regclass
        and contype = 'c'
        and pg_get_constraintdef(oid) like '%entity_type%asset%liability%case%'
        and pg_get_constraintdef(oid) not like '%reservation%'
    loop
        execute format('alter table domain_events drop constraint %I', constraint_name);
    end loop;
end $$;

do $$ begin
    if not exists (select 1 from pg_constraint where conrelid = 'domain_events'::regclass and conname = 'domain_events_entity_type_check') then
        alter table domain_events add constraint domain_events_entity_type_check
            check (entity_type in ('asset', 'liability', 'case', 'reservation'));
    end if;
end $$;

do $$ begin
    if not exists (select 1 from pg_constraint where conrelid = 'domain_events'::regclass and conname = 'domain_events_related_entity_type_check') then
        alter table domain_events add constraint domain_events_related_entity_type_check
            check (related_entity_type is null or related_entity_type in ('document', 'transaction', 'asset', 'liability', 'case', 'reservation'));
    end if;
end $$;

insert into domain_events (entity_type, entity_id, event_type, event_date, description, created_at)
select 'reservation', id, 'reservation_created', (created_at at time zone 'UTC')::date, 'Reservering aangemaakt', created_at
from reservations
where not exists (
    select 1 from domain_events
    where entity_type = 'reservation' and entity_id = reservations.id and event_type = 'reservation_created'
);

with duplicates as (
    select id, row_number() over (partition by entity_type, entity_id, event_type order by id) as row_number
    from domain_events
    where event_type in ('asset_created', 'liability_created', 'case_created', 'reservation_created')
)
delete from domain_events using duplicates
where domain_events.id = duplicates.id and duplicates.row_number > 1;

drop index if exists domain_events_creation_unique_idx;
create unique index domain_events_creation_unique_idx on domain_events(entity_type, entity_id)
where event_type in ('asset_created', 'liability_created', 'case_created', 'reservation_created');

insert into schema_migrations (version) values ('011_reservations')
on conflict (version) do nothing;
