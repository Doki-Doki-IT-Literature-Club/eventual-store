package shared

import (
	"context"
	"fmt"
	"log"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

const dbConnString = "postgres://user:password@postgres:5432/"

func EnsureDBExists(ctx context.Context, dbName string) error {
	conn, err := ConnectToDB(ctx, dbName)
	if err != nil {
		return err
	}
	defer conn.Close(ctx)

	_, err = conn.Exec(ctx, fmt.Sprintf("CREATE DATABASE %s", pgx.Identifier{dbName}.Sanitize()))
	if err != nil {
		pgErr, ok := err.(*pgconn.PgError)
		// 42P04 = duplicate_database'
		if ok && pgErr.Code == "42P04" {
			log.Printf("Database '%s' exists.\n", dbName)
			return nil
		} else {
			return fmt.Errorf("error creating database '%s': %v", dbName, err)
		}
	}

	log.Printf("Database '%s' created.\n", dbName)
	return nil
}

func ConnectToDB(ctx context.Context, dbname string) (*pgx.Conn, error) {
	conn, err := pgx.Connect(ctx, dbConnString+dbname)

	if err != nil {
		return nil, fmt.Errorf("unable to connect to database: %v", err)
	}

	return conn, nil
}

func InitDataBases(ctx context.Context, conn *pgx.Conn, sql []string) error {
	tx, err := conn.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction for schema init: %w", err)
	}
	defer tx.Rollback(ctx)

	for _, query := range sql {
		_, err = tx.Exec(ctx, query)
		if err != nil {
			return fmt.Errorf("unable to execute query: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit schema init transaction: %w", err)
	}

	log.Println("Database schema initialization complete.")
	return nil
}
