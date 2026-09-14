alter table cases add column if not exists main_category_id bigint;

do $$ begin
    if not exists (select 1 from pg_constraint where conname = 'cases_main_category_fk') then
        alter table cases add constraint cases_main_category_fk
            foreign key (main_category_id) references main_categories(id) on delete cascade;
    end if;
end $$;

create unique index if not exists cases_main_category_unique_idx
    on cases(main_category_id) where main_category_id is not null;

insert into cases (household_id, main_category_id, case_type, title, status, created_at, updated_at)
select (select id from households order by id limit 1), main_categories.id, 'main_category', main_categories.name, 'active', main_categories.created_at, main_categories.created_at
from main_categories
where not exists (select 1 from cases where cases.main_category_id = main_categories.id);

insert into domain_events (entity_type, entity_id, event_type, event_date, description, created_at)
select 'case', cases.id, 'case_created', (cases.created_at at time zone 'UTC')::date, 'Dossier aangemaakt', cases.created_at
from cases where cases.main_category_id is not null
on conflict do nothing;

insert into schema_migrations (version) values ('009_main_category_cases')
on conflict (version) do nothing;
