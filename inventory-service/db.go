package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/Doki-Doki-IT-Literature-Club/sops/shared"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type InventoryItem struct {
	ID     string `json:"id"`
	Amount int    `json:"amount"`
}

const orderRequestResultOutboxEventType = "OrderRequestResult"
const inventoryItemOutboxEventType = "InventoryItem"

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

// todo: bulk update
func updateInventoryItems(conn *pgx.Conn, ctx context.Context, items []InventoryItem) error {
	tx, err := conn.Begin(ctx)
	if err != nil {
		return fmt.Errorf("unable to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	for _, item := range items {
		err := updateInventoryItem(conn, ctx, item)
		if err != nil {
			return fmt.Errorf("error updating inventory item: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("unable to commit transaction: %w", err)
	}

	log.Printf("Updated inventory items: %v\n", items)
	return nil
}

func updateInventoryItem(conn *pgx.Conn, ctx context.Context, item InventoryItem) error {
	query := `
	INSERT INTO inventory (id, amount) 
	VALUES ($1, $2) 
	ON CONFLICT (id)
	DO UPDATE SET amount = $2;
	`

	tx, err := conn.Begin(ctx)
	if err != nil {
		return fmt.Errorf("unable to begin transaction: %w", err)
	}

	defer tx.Rollback(ctx)
	_, err = tx.Exec(ctx, query, item.ID, item.Amount)
	if err != nil {
		return fmt.Errorf("unable to update inventory item: %w", err)
	}

	itemBytes, err := json.Marshal(item)
	if err != nil {
		return fmt.Errorf("unable to marshal item: %w", err)
	}
	err = shared.InsertOutboxEvent(ctx, tx, "InventoryItem", item.ID, inventoryItemOutboxEventType, itemBytes)
	if err != nil {
		return fmt.Errorf("unable to insert outbox event: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("unable to commit transaction: %w", err)
	}
	log.Printf("Updated inventory item: %v\n", item)
	return nil
}

func getInventoryItems(conn *pgx.Conn, ctx context.Context, productID []string) ([]InventoryItem, error) {
	query := `
	SELECT id, amount
	FROM inventory
	WHERE id = ANY($1);
	`
	rows, err := conn.Query(ctx, query, productID)
	if err != nil {
		return nil, fmt.Errorf("unable to query inventory: %w", err)
	}
	defer rows.Close()
	var items []InventoryItem
	for rows.Next() {
		var item InventoryItem
		if err := rows.Scan(&item.ID, &item.Amount); err != nil {
			return nil, fmt.Errorf("unable to scan row: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}

	log.Printf("Fetched inventory items: %v\n", items)
	return items, nil
}

func fillDB(conn *pgx.Conn, ctx context.Context) error {
	products := []InventoryItem{
		{ID: "product1", Amount: 100},
		{ID: "product2", Amount: 2},
		{ID: "product3", Amount: 0},
	}

	for _, item := range products {
		err := updateInventoryItem(conn, ctx, item)
		if err != nil {
			return fmt.Errorf("error filling DB: %w", err)
		}
	}

	log.Println("Database filled with initial product data.")
	log.Printf("Products: %v\n", products)
	return nil
}
