alter table cases add column if not exists term_months integer;

do $$ begin
    if not exists (select 1 from pg_constraint where conname = 'cases_term_months_check') then
        alter table cases add constraint cases_term_months_check
            check (term_months is null or term_months > 0);
    end if;
end $$;

insert into schema_migrations (version) values ('010_case_contract_fields')
on conflict (version) do nothing;
