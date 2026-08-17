create table if not exists documents (
    id bigserial primary key,
    original_filename text not null,
    mime_type text not null,
    size_bytes bigint not null check (size_bytes >= 0),
    sha256 char(64) not null,
    object_key text not null unique,
    document_date date,
    description text,
    created_at timestamptz not null default now()
);

create index if not exists documents_sha256_idx on documents(sha256);

create table if not exists document_links (
    document_id bigint not null references documents(id) on delete cascade,
    object_type text not null,
    object_id bigint not null,
    created_at timestamptz not null default now(),
    primary key (document_id, object_type, object_id)
);

create index if not exists document_links_object_idx on document_links(object_type, object_id);
