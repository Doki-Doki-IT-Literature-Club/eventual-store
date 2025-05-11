package main

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5"
)

const (
	dbName = "showcase_db"
)

type Order struct {
	ID       string         `json:"id"`
	Status   string         `json:"status"`
	Products map[string]int `json:"products"`
}

type Inventory struct {
	ID     string `json:"id"`
	Amount int    `json:"amount"`
}

func initDB(conn *pgx.Conn) error {
	tx, err := conn.BeginTx(context.Background(), pgx.TxOptions{})

	if err != nil {
		return fmt.Errorf("unable to start transaction: %v", err)
	}
	defer tx.Rollback(context.Background())

	ordersTableSQL := `
		CREATE TABLE IF NOT EXISTS orders
	  (id TEXT PRIMARY KEY, status TEXT, products TEXT)`

	inventoryTableSQL := `
		CREATE TABLE IF NOT EXISTS inventory
	  (id TEXT PRIMARY KEY, amount NUMERIC)`

	_, err = tx.Exec(context.Background(), ordersTableSQL)

	if err != nil {
		return fmt.Errorf("unable to create table: %v", err)
	}

	_, err = tx.Exec(context.Background(), inventoryTableSQL)
	if err != nil {
		return fmt.Errorf("unable to create table: %v", err)
	}

	err = tx.Commit(context.Background())

	return nil
}

func updateOrder(conn *pgx.Conn, ctx context.Context, order Order) error {
	query := `
	INSERT INTO orders (id, status, products) 
	VALUES ($1, $2, $3) 
	ON CONFLICT (id)
	DO UPDATE SET status = $2, products = $3;
	`
	stringProducs, err := json.Marshal(order.Products)
	_, err = conn.Exec(context.Background(), query, order.ID, order.Status, stringProducs)
	if err != nil {
		return fmt.Errorf("unable to update: %v", err)
	}
	return nil
}

func getOrders(conn *pgx.Conn, ctx context.Context) ([]Order, error) {
	selectSQL := `
	SELECT id, products, status
	FROM orders 
	`
	rows, err := conn.Query(ctx, selectSQL)
	if err != nil {
		return nil, fmt.Errorf("error querying orders: %w", err)
	}
	defer rows.Close()

	orders := make([]Order, 0, 0)
	for rows.Next() {
		var order Order
		var products string
		var productsMap map[string]int
		if err := rows.Scan(&order.ID, &products, &order.Status); err != nil {
			return nil, fmt.Errorf("error scanning row: %w", err)
		}
		err := json.Unmarshal([]byte(products), &productsMap)
		if err != nil {
			return nil, fmt.Errorf("error formatting products: %w", err)
		}
		order.Products = productsMap
		orders = append(orders, order)
	}
	return orders, nil
}

func getInventory(conn *pgx.Conn, ctx context.Context) ([]Inventory, error) {
	selectSQL := `
	SELECT id, amount
	FROM inventory 
	`
	rows, err := conn.Query(ctx, selectSQL)
	if err != nil {
		return nil, fmt.Errorf("error querying inventory: %w", err)
	}
	defer rows.Close()

	inventories := make([]Inventory, 0, 0)
	for rows.Next() {
		var inventory Inventory
		if err := rows.Scan(&inventory.ID, &inventory.Amount); err != nil {
			return nil, fmt.Errorf("error scanning row: %w", err)
		}
		inventories = append(inventories, inventory)
	}
	return inventories, nil
}

func updateInventory(conn *pgx.Conn, ctx context.Context, inventory Inventory) error {
	query := `
	INSERT INTO inventory (id, amount) 
	VALUES ($1, $2) 
	ON CONFLICT (id)
	DO UPDATE SET amount = $2;
	`
	_, err := conn.Exec(context.Background(), query, inventory.ID, inventory.Amount)
	if err != nil {
		return fmt.Errorf("unable to update: %v", err)
	}
	return nil
}
