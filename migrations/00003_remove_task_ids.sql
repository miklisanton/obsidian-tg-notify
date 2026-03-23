-- +goose Up
drop index if exists task_snapshots_task_id_uidx;

alter table task_snapshots
    drop column if exists task_id;

-- +goose Down
alter table task_snapshots
    add column if not exists task_id text;

create unique index if not exists task_snapshots_task_id_uidx
    on task_snapshots (vault_id, task_id)
    where task_id is not null;
