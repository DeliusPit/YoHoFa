begin;

alter table transactions add column if not exists counterparty_party_id bigint references parties(id) on delete set null;
alter table transactions add column if not exists counterparty_name_raw text;
alter table transactions add column if not exists counterparty_iban_raw text;

update transactions
set counterparty_name_raw = counterparty
where counterparty_name_raw is null and btrim(coalesce(counterparty, '')) <> '';

update transactions
set counterparty_iban_raw = counterparty_iban
where counterparty_iban_raw is null and btrim(coalesce(counterparty_iban, '')) <> '';

insert into parties (household_id, party_type, name)
select distinct accounts.household_id, 'organization', btrim(transactions.counterparty_name_raw)
from transactions
join accounts on accounts.id = transactions.account_id
where btrim(coalesce(transactions.counterparty_name_raw, '')) <> ''
and not exists (
    select 1 from parties
    where parties.household_id = accounts.household_id
    and lower(parties.name) = lower(btrim(transactions.counterparty_name_raw))
);

update transactions
set counterparty_party_id = parties.id
from accounts, parties
where transactions.counterparty_party_id is null
and accounts.id = transactions.account_id
and btrim(coalesce(transactions.counterparty_name_raw, '')) <> ''
and parties.household_id = accounts.household_id
and lower(parties.name) = lower(btrim(transactions.counterparty_name_raw));

insert into party_identifiers (household_id, party_id, identifier_type, identifier_value, normalized_value)
select distinct accounts.household_id, transactions.counterparty_party_id, 'iban', btrim(transactions.counterparty_iban_raw), upper(replace(btrim(transactions.counterparty_iban_raw), ' ', ''))
from transactions
join accounts on accounts.id = transactions.account_id
where transactions.counterparty_party_id is not null
and btrim(coalesce(transactions.counterparty_iban_raw, '')) <> ''
on conflict (household_id, identifier_type, normalized_value) do nothing;

create index if not exists transactions_counterparty_party_idx on transactions(counterparty_party_id, booked_on desc) where counterparty_party_id is not null;

insert into schema_migrations (version) values ('014_transaction_counterparties')
on conflict (version) do nothing;

commit;
