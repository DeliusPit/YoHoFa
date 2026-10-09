-- HoHoHo target schema v3 — de eenvoudige kern (SQLite)
-- Negen tabellen: financiële objecten, transacties, documenten, koppelingen,
-- waardehistorie, bankimportprofielen, ruwe importregels en objectrelaties.
-- Objecttypes: asset, debt, reserve, expense, financial; validatie in de applicatie.
-- Eén deployment/database vertegenwoordigt één huishouden.
-- SQLite: INTEGER PRIMARY KEY voor IDs, TEXT voor ISO-datums en JSON.
-- Canonieke v3-schema-initialisatie, ingebed in de SQLite-store.
-- De store regelt foreign keys, transactie en PRAGMA user_version.

CREATE TABLE financial_objects (
    id          INTEGER PRIMARY KEY,
    object_type TEXT NOT NULL,
    name        TEXT NOT NULL,
    description TEXT
);

CREATE TABLE transactions (
    id               INTEGER PRIMARY KEY,
    transaction_date TEXT NOT NULL,
    amount_cents     INTEGER NOT NULL,
    description      TEXT,
    counterparty     TEXT,
    status           TEXT NOT NULL DEFAULT 'I' CHECK (status IN ('I', 'A', 'F', 'U')),
    hidden           INTEGER NOT NULL DEFAULT 0 CHECK (hidden IN (0, 1))
);

CREATE TABLE transaction_financial_objects (
    transaction_id      INTEGER NOT NULL REFERENCES transactions(id) ON DELETE CASCADE,
    financial_object_id INTEGER NOT NULL REFERENCES financial_objects(id),
    amount_cents        INTEGER,

    PRIMARY KEY (
        transaction_id,
        financial_object_id
    )
);

CREATE TABLE documents (
    id              INTEGER PRIMARY KEY,
    filename        TEXT NOT NULL,
    object_key      TEXT NOT NULL UNIQUE,
    mime_type       TEXT NOT NULL DEFAULT '',
    size_bytes      INTEGER NOT NULL DEFAULT 0 CHECK (size_bytes >= 0),
    sha256          TEXT NOT NULL DEFAULT '',
    status          TEXT NOT NULL DEFAULT 'I' CHECK (status IN ('I', 'A', 'F', 'U')),
    created_at      TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
    document_date   TEXT,
    description     TEXT,
    extracted_text  TEXT,
    translated_text TEXT,
    analysis_json   TEXT CHECK (analysis_json IS NULL OR json_valid(analysis_json))
);

CREATE TABLE document_financial_objects (
    document_id         INTEGER NOT NULL REFERENCES documents(id) ON DELETE CASCADE,
    financial_object_id INTEGER NOT NULL REFERENCES financial_objects(id),

    PRIMARY KEY (
        document_id,
        financial_object_id
    )
);

CREATE TABLE object_values (
    id                  INTEGER PRIMARY KEY,
    financial_object_id INTEGER NOT NULL REFERENCES financial_objects(id) ON DELETE CASCADE,
    value_date          TEXT NOT NULL,
    value_cents         INTEGER NOT NULL
);

CREATE TABLE bank_import_profiles (
    id            INTEGER PRIMARY KEY,
    name          TEXT NOT NULL,
    source_format TEXT NOT NULL,
    configuration TEXT NOT NULL CHECK (json_valid(configuration))
);

CREATE TABLE bank_import_rows (
    id             INTEGER PRIMARY KEY,
    profile_id     INTEGER NOT NULL REFERENCES bank_import_profiles(id) ON DELETE CASCADE,
    raw_data       TEXT NOT NULL CHECK (json_valid(raw_data)),
    fingerprint    TEXT NOT NULL,
    transaction_id INTEGER REFERENCES transactions(id) ON DELETE SET NULL
);

CREATE UNIQUE INDEX bank_import_rows_profile_fingerprint_uidx
    ON bank_import_rows(profile_id, fingerprint);

CREATE TABLE object_relations (
    source_object_id INTEGER NOT NULL REFERENCES financial_objects(id) ON DELETE CASCADE,
    target_object_id INTEGER NOT NULL REFERENCES financial_objects(id) ON DELETE CASCADE,
    relation_type    TEXT NOT NULL,

    PRIMARY KEY (
        source_object_id,
        target_object_id,
        relation_type
    )
);
