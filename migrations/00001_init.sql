-- +goose Up
create table if not exists vaults (
    id bigint primary key,
    name text not null unique,
    root_path text not null unique,
    timezone text not null
);

create table if not exists telegram_chats (
    chat_id bigint primary key,
    created_at timestamptz not null default now()
);

create table if not exists reminder_rules (
    id text primary key,
    chat_id bigint not null references telegram_chats(chat_id),
    vault_id bigint not null references vaults(id),
    kind text not null,
    timezone text not null,
    schedule_json jsonb not null,
    config_json jsonb not null,
    enabled boolean not null default true,
    created_at timestamptz not null,
    updated_at timestamptz not null
);

create table if not exists document_snapshots (
    vault_id bigint not null references vaults(id),
    source_path text not null,
    source_kind text not null,
    daily_date date,
    iso_week_year integer,
    iso_week_number integer,
    weekly_area text,
    has_non_blank_task boolean not null,
    synced_at timestamptz not null,
    primary key (vault_id, source_path)
);

create table if not exists task_snapshots (
    vault_id bigint not null references vaults(id),
    source_path text not null,
    source_kind text not null,
    line_number integer not null,
    section text not null,
    fingerprint text not null,
    body text not null,
    completed boolean not null,
    due_date date,
    done_date date,
    daily_date date,
    iso_week_year integer,
    iso_week_number integer,
    weekly_area text,
    seen_at timestamptz not null,
    updated_at timestamptz not null,
    primary key (vault_id, source_path, fingerprint)
);

create index if not exists task_snapshots_due_idx on task_snapshots (vault_id, due_date) where due_date is not null and completed = false;
create index if not exists task_snapshots_week_idx on task_snapshots (vault_id, iso_week_year, iso_week_number) where source_kind = 'weekly';
create index if not exists document_snapshots_daily_idx on document_snapshots (vault_id, daily_date) where daily_date is not null;
create index if not exists document_snapshots_week_idx on document_snapshots (vault_id, iso_week_year, iso_week_number) where source_kind = 'weekly';

create table if not exists sent_notifications (
    dedupe_key text primary key,
    chat_id bigint not null,
    rule_id text,
    kind text not null,
    payload jsonb not null,
    sent_at timestamptz not null
);

-- +goose Down
drop table if exists sent_notifications;
drop table if exists task_snapshots;
drop table if exists document_snapshots;
drop table if exists reminder_rules;
drop table if exists telegram_chats;
drop table if exists vaults;
