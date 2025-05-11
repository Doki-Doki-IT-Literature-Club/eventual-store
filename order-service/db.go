package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/Doki-Doki-IT-Literature-Club/sops/shared"

	"github.com/jackc/pgx/v5"
)

func upsertOrder(ctx context.Context, conn *pgx.Conn, order *Order) error {
	log.Print("Starting upserting")
	tx, err := conn.Begin(ctx)
	if err != nil {
		return fmt.Errorf("Could not begin transaction: %s", err.Error())
	}
	defer tx.Rollback(ctx)
	updateOrderQ := `
    INSERT INTO orders (id, status, products) 
    VALUES ($1, $2, $3)

    ON CONFLICT(id)
    DO UPDATE SET status=$2
    `

	productBytes, err := json.Marshal(order.Products)
	if err != nil {
		return err
	}

	_, err = tx.Exec(context.Background(), updateOrderQ, order.ID, order.Status, productBytes)
	if err != nil {
		return fmt.Errorf("unable to upsert: %v", err)
	}

	data, err := json.Marshal(order)
	if err != nil {
		return err
	}

	err = shared.InsertOutboxEvent(ctx, tx, "Order", order.ID, orderUpdateOutboxEventType, data)
	if err != nil {
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("unable to commit: %v", err)
	}
	return nil
}

func initDB(ctx context.Context, conn *pgx.Conn) error {
	ordersTableSQL := `CREATE TABLE IF NOT EXISTS orders (id TEXT PRIMARY KEY, status TEXT, products TEXT)`
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

	return shared.InitDataBases(ctx, conn, []string{ordersTableSQL, outboxTableSQL, outboxIndexSQL})
}
