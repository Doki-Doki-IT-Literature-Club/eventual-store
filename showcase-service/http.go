package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/jackc/pgx/v5"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func httpServer(conn *pgx.Conn) {
	mux := http.NewServeMux()

	mux.Handle("/metrics", promhttp.Handler())
	mux.Handle("GET /orders", prometheusMiddleware(handleGetOrdersTableView(conn)))
	mux.Handle("GET /inventory", prometheusMiddleware(handleGetInventoryTableView(conn)))
	mux.Handle("POST /api/inventory/order", prometheusMiddleware(handlePostInventoryOrder()))

	port := 80
	log.Printf("Starting server on port %d", port)
	if err := http.ListenAndServe(fmt.Sprintf(":%d", port), mux); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}
