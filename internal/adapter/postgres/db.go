package postgres

import (
	"context"
	"fmt"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/jmoiron/sqlx"
	"github.com/pressly/goose/v3"
)

type DB struct {
	*sqlx.DB
}

func Open(ctx context.Context, dsn string) (*DB, error) {
	var lastErr error
	for attempt := 1; attempt <= 30; attempt++ {
		sqlxDB, err := sqlx.ConnectContext(ctx, "pgx", dsn)
		if err == nil {
			if pingErr := sqlxDB.PingContext(ctx); pingErr == nil {
				return &DB{DB: sqlxDB}, nil
			} else {
				lastErr = fmt.Errorf("ping db: %w", pingErr)
				_ = sqlxDB.Close()
			}
		} else {
			lastErr = fmt.Errorf("connect db: %w", err)
		}

		if attempt == 30 {
			break
		}

		select {
		case <-ctx.Done():
			return nil, lastErr
		case <-time.After(time.Second):
		}
	}

	return nil, lastErr
}

func Migrate(db *sqlx.DB, dir string) error {
	if err := goose.SetDialect("postgres"); err != nil {
		return err
	}
	return goose.Up(db.DB, dir)
}
