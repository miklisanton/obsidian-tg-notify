package postgres

import (
	"context"
	"fmt"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/jmoiron/sqlx"
	"github.com/pressly/goose/v3"
)

type DB struct {
	*sqlx.DB
}

func Open(ctx context.Context, dsn string) (*DB, error) {
	sqlxDB, err := sqlx.ConnectContext(ctx, "pgx", dsn)
	if err != nil {
		return nil, fmt.Errorf("connect db: %w", err)
	}
	if err := sqlxDB.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("ping db: %w", err)
	}
	return &DB{DB: sqlxDB}, nil
}

func Migrate(db *sqlx.DB, dir string) error {
	if err := goose.SetDialect("postgres"); err != nil {
		return err
	}
	return goose.Up(db.DB, dir)
}
