begin;
select pg_advisory_xact_lock(684523817);

create table if not exists domain_events (
    id bigserial primary key,
    entity_type text not null check (entity_type in ('asset', 'liability', 'case')),
    entity_id bigint not null check (entity_id > 0),
    event_type text not null check (btrim(event_type) <> ''),
    event_date date not null,
    description text not null check (btrim(description) <> ''),
    related_entity_type text,
    related_entity_id bigint,
    created_at timestamptz not null default now(),
    check ((related_entity_type is null) = (related_entity_id is null)),
    check (related_entity_type is null or related_entity_type in ('document', 'transaction', 'asset', 'liability', 'case')),
    check (related_entity_id is null or related_entity_id > 0),
    check (related_entity_type is null or entity_type <> related_entity_type or entity_id <> related_entity_id)
);

create index if not exists domain_events_timeline_idx on domain_events(entity_type, entity_id, event_date desc, id desc);
create unique index if not exists domain_events_creation_unique_idx on domain_events(entity_type, entity_id)
where event_type in ('asset_created', 'liability_created', 'case_created');

insert into domain_events (entity_type, entity_id, event_type, event_date, description, created_at)
select 'asset', id, 'asset_created', (created_at at time zone 'UTC')::date, 'Bezitting aangemaakt', created_at from assets
on conflict do nothing;
insert into domain_events (entity_type, entity_id, event_type, event_date, description, created_at)
select 'liability', id, 'liability_created', (created_at at time zone 'UTC')::date, 'Schuld aangemaakt', created_at from liabilities
on conflict do nothing;
insert into domain_events (entity_type, entity_id, event_type, event_date, description, created_at)
select 'case', id, 'case_created', (created_at at time zone 'UTC')::date, 'Dossier aangemaakt', created_at from cases
on conflict do nothing;

insert into schema_migrations (version) values ('008_domain_events') on conflict (version) do nothing;

commit;
