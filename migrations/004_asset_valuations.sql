create table if not exists asset_valuations (
    id bigserial primary key,
    asset_id bigint not null references assets(id) on delete cascade,
    valuation_date date not null,
    value bigint not null check (value >= 0),
    valuation_type text not null check (valuation_type in ('purchase', 'estimate', 'appraisal', 'tax', 'market', 'other')),
    source text,
    notes text,
    created_at timestamptz not null default now()
);

create index if not exists asset_valuations_asset_date_idx on asset_valuations(asset_id, valuation_date desc, id desc);

insert into asset_valuations (asset_id, valuation_date, value, valuation_type, source)
select assets.id, coalesce(assets.acquired_at, current_date), assets.current_value, 'estimate', 'Migratie huidige waarde'
from assets
where assets.current_value > 0
and not exists (select 1 from asset_valuations where asset_valuations.asset_id = assets.id);

update assets
set current_value = coalesce((
    select asset_valuations.value
    from asset_valuations
    where asset_valuations.asset_id = assets.id
    order by asset_valuations.valuation_date desc, asset_valuations.id desc
    limit 1
), 0);
