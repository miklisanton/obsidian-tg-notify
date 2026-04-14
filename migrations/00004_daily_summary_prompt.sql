-- +goose Up
alter table document_snapshots
    add column if not exists has_daily_summary boolean not null default false;

create table if not exists telegram_messages (
    chat_id bigint not null references telegram_chats(chat_id),
    telegram_message_id integer not null,
    text text not null,
    sent_at timestamptz not null,
    created_at timestamptz not null default now(),
    primary key (chat_id, telegram_message_id)
);

create index if not exists telegram_messages_chat_sent_idx
    on telegram_messages (chat_id, sent_at);

-- +goose Down
drop index if exists telegram_messages_chat_sent_idx;

drop table if exists telegram_messages;

alter table document_snapshots
    drop column if exists has_daily_summary;
