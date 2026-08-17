begin;
select pg_advisory_xact_lock(684523817);

create table if not exists schema_migrations (
    version text primary key,
    applied_at timestamptz not null default now()
);

create table if not exists entity_relations (
    id bigserial primary key,
    source_type text not null,
    source_id bigint not null,
    target_type text not null,
    target_id bigint not null,
    relation_type text not null check (relation_type in ('relates_to', 'evidence_for')),
    valid_from date,
    valid_to date,
    notes text,
    created_at timestamptz not null default now(),
    check (source_type <> target_type or source_id <> target_id),
    check (valid_to is null or valid_from is null or valid_to >= valid_from)
);

create unique index if not exists entity_relations_unique_idx on entity_relations(
    source_type, source_id, target_type, target_id, relation_type,
    coalesce(valid_from, '-infinity'::date), coalesce(valid_to, 'infinity'::date)
);
create index if not exists entity_relations_source_idx on entity_relations(source_type, source_id, relation_type);
create index if not exists entity_relations_target_idx on entity_relations(target_type, target_id, relation_type);

insert into entity_relations (source_type, source_id, target_type, target_id, relation_type, created_at)
select 'asset', legacy.asset_id, 'transaction', legacy.transaction_id, 'relates_to', min(legacy.created_at)
from (
    select object_id as asset_id, related_object_id as transaction_id, created_at
    from object_relations
    where object_type = 'asset' and related_object_type = 'transaction'
    union all
    select related_object_id, object_id, created_at
    from object_relations
    where object_type = 'transaction' and related_object_type = 'asset'
) legacy
join assets on assets.id = legacy.asset_id
join transactions on transactions.id = legacy.transaction_id
where not exists (select 1 from schema_migrations where version = '005_entity_relations')
and not exists (
    select 1 from entity_relations
    where source_type = 'asset'
      and source_id = legacy.asset_id
      and target_type = 'transaction'
      and target_id = legacy.transaction_id
      and relation_type = 'relates_to'
      and valid_from is null and valid_to is null
)
group by legacy.asset_id, legacy.transaction_id;

insert into entity_relations (source_type, source_id, target_type, target_id, relation_type, created_at)
select 'document', legacy.document_id, legacy.target_type, legacy.target_id, 'evidence_for', min(legacy.created_at)
from (
    select document_links.document_id, 'asset'::text as target_type, document_links.object_id as target_id, document_links.created_at
    from document_links
    join documents on documents.id = document_links.document_id
    join assets on assets.id = document_links.object_id
    where document_links.object_type = 'asset'
    union all
    select document_links.document_id, 'transaction', document_links.object_id, document_links.created_at
    from document_links
    join documents on documents.id = document_links.document_id
    join transactions on transactions.id = document_links.object_id
    where document_links.object_type = 'transaction'
) legacy
where not exists (select 1 from schema_migrations where version = '005_entity_relations')
and not exists (
    select 1 from entity_relations
    where source_type = 'document'
      and source_id = legacy.document_id
      and target_type = legacy.target_type
      and target_id = legacy.target_id
      and relation_type = 'evidence_for'
      and valid_from is null and valid_to is null
)
group by legacy.document_id, legacy.target_type, legacy.target_id;

insert into schema_migrations (version) values ('005_entity_relations') on conflict (version) do nothing;

commit;
