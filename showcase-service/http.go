package main

import (
	"context"
	"fmt"
	"log"
	"net/http"

	"github.com/jackc/pgx/v5"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/twmb/franz-go/pkg/kgo"
)

func httpServer(conn *pgx.Conn, kcl *kgo.Client, ctx context.Context) {
	mux := http.NewServeMux()

	mux.Handle("/metrics", promhttp.Handler())
	mux.Handle("GET /orders", prometheusMiddleware(handleGetOrdersTableView(conn)))
	mux.Handle("GET /inventory", prometheusMiddleware(handleGetInventoryTableView(conn)))

	port := 80
	log.Printf("Starting server on port %d", port)
	if err := http.ListenAndServe(fmt.Sprintf(":%d", port), mux); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}
