begin;

create table if not exists parties (
    id bigserial primary key,
    household_id bigint not null default 1 references households(id) on delete cascade,
    party_type text not null default 'organization'
        check (party_type in ('person','organization','bank','government','insurer','supplier','other')),
    name text not null check (btrim(name) <> ''),
    notes text,
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now(),
    unique (household_id, id)
);

create index if not exists parties_household_name_idx on parties(household_id, lower(name));

create table if not exists party_identifiers (
    id bigserial primary key,
    household_id bigint not null default 1 references households(id) on delete cascade,
    party_id bigint not null references parties(id) on delete cascade,
    identifier_type text not null
        check (identifier_type in ('iban','bic','creditor_id','vat_id','tax_id','customer_number','email','phone','other')),
    identifier_value text not null check (btrim(identifier_value) <> ''),
    normalized_value text not null check (btrim(normalized_value) <> ''),
    created_at timestamptz not null default now(),
    unique (household_id, identifier_type, normalized_value)
);

create index if not exists party_identifiers_party_idx on party_identifiers(party_id);

alter table liabilities add column if not exists creditor_party_id bigint references parties(id) on delete set null;
create index if not exists liabilities_creditor_party_idx on liabilities(creditor_party_id) where creditor_party_id is not null;

insert into parties (household_id, party_type, name)
select distinct liabilities.household_id, 'organization', btrim(liabilities.creditor)
from liabilities
where btrim(coalesce(liabilities.creditor, '')) <> ''
and not exists (
    select 1 from parties
    where parties.household_id = liabilities.household_id
    and lower(parties.name) = lower(btrim(liabilities.creditor))
);

update liabilities
set creditor_party_id = parties.id
from parties
where liabilities.creditor_party_id is null
and btrim(coalesce(liabilities.creditor, '')) <> ''
and parties.household_id = liabilities.household_id
and lower(parties.name) = lower(btrim(liabilities.creditor));

create table if not exists document_analyses (
    id bigserial primary key,
    document_id bigint not null references documents(id) on delete cascade,
    analysis_type text not null check (analysis_type in ('translation','analysis')),
    status text not null default 'pending'
        check (status in ('not_requested','pending','completed','unsupported','failed')),
    language text,
    model text,
    content_text text,
    content_json jsonb,
    error_message text,
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now(),
    unique (document_id, analysis_type)
);

insert into document_analyses (document_id, analysis_type, status, language, content_text, created_at, updated_at)
select id, 'translation', translation_status, 'nl', nullif(translation_text, ''), created_at, now()
from documents
where translation_status <> 'not_requested' or btrim(coalesce(translation_text, '')) <> ''
on conflict (document_id, analysis_type) do update
set status = excluded.status,
    language = coalesce(document_analyses.language, excluded.language),
    content_text = coalesce(nullif(document_analyses.content_text, ''), excluded.content_text),
    updated_at = now();

insert into schema_migrations (version) values ('013_document_analyses_parties')
on conflict (version) do nothing;

commit;
