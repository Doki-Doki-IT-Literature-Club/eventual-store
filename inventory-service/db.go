package main

import (
	"context"
	"fmt"
	"log"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

const dbName = "inventory"
const dbConnString = "postgres://user:password@postgres:5432/"

func ensureDBExists(dbName string) {
	dbConnString := dbConnString + "postgres"
	conn, err := pgx.Connect(context.Background(), dbConnString)
	if err != nil {
		log.Fatalf("Unable to connect to database: %v", err)
	}
	defer conn.Close(context.Background())

	_, err = conn.Exec(context.Background(), fmt.Sprintf("CREATE DATABASE %s", pgx.Identifier{dbName}.Sanitize()))
	if err != nil {
		pgErr, ok := err.(*pgconn.PgError)
		// 42P04 = duplicate_database'
		if ok && pgErr.Code == "42P04" {
			log.Printf("Database '%s' exists.\n", dbName)
		} else {
			log.Fatalf("Failed to create database '%s': %v\n", dbName, err)
		}
	} else {
		log.Printf("Database '%s' created.\n", dbName)
	}

	log.Printf("Database '%s' checked.\n", dbName)
}

func connectToDB() (*pgx.Conn, error) {
	conn, err := pgx.Connect(context.Background(), dbConnString+dbName)
	if err != nil {
		return nil, fmt.Errorf("unable to connect to database: %w", err)
	}
	return conn, nil
}

func initDB(conn *pgx.Conn) error {
	inventoryTableSQL := `
	CREATE TABLE IF NOT EXISTS inventory (
		product_id TEXT PRIMARY KEY,
		amount INT NOT NULL CHECK (amount >= 0)
	);`

	outboxTableSQL := `
	CREATE TABLE IF NOT EXISTS outbox_events (
		id UUID PRIMARY KEY,
		aggregate_type TEXT NOT NULL,
		aggregate_id TEXT NOT NULL,
		event_type TEXT NOT NULL,
		payload JSONB NOT NULL,
		status VARCHAR(20) NOT NULL DEFAULT 'PENDING', -- PENDING, PUBLISHED, FAILED
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		published_at TIMESTAMPTZ NULL
	);`

	outboxIndexSQL := `
	CREATE INDEX IF NOT EXISTS idx_outbox_events_status_created_at
	ON outbox_events (status, created_at);
	`

	ctx := context.Background()
	tx, err := conn.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction for schema init: %w", err)
	}
	defer tx.Rollback(ctx)

	_, err = tx.Exec(ctx, inventoryTableSQL)
	if err != nil {
		return fmt.Errorf("unable to create inventory table: %w", err)
	}
	log.Println("Inventory table checked/created.")

	_, err = tx.Exec(ctx, outboxTableSQL)
	if err != nil {
		return fmt.Errorf("unable to create outbox_events table: %w", err)
	}
	log.Println("Outbox events table checked/created.")

	_, err = tx.Exec(ctx, outboxIndexSQL)
	if err != nil {
		return fmt.Errorf("unable to create index on outbox_events table: %w", err)
	}
	log.Println("Outbox events index checked/created.")

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit schema init transaction: %w", err)
	}

	log.Println("Database schema initialization complete.")
	return nil
}
