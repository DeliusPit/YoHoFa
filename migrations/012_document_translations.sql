alter table documents add column if not exists translation_text text;
alter table documents add column if not exists translation_status text not null default 'not_requested';

do $$ begin
    if not exists (select 1 from pg_constraint where conname = 'documents_translation_status_check') then
        alter table documents add constraint documents_translation_status_check
            check (translation_status in ('not_requested', 'pending', 'completed', 'unsupported', 'failed'));
    end if;
end $$;

insert into schema_migrations (version) values ('012_document_translations')
on conflict (version) do nothing;
