package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/Doki-Doki-IT-Literature-Club/sops/shared"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/twmb/franz-go/pkg/kgo"
)

func httpServer(conn *pgx.Conn, kcl *kgo.Client, ctx context.Context) {
	http.HandleFunc("POST /api/orders", func(w http.ResponseWriter, r *http.Request) {
		log.Printf("Received request: %s %s", r.Method, r.URL.Path)
		payload := shared.CreateOrderPayload{}
		if json.NewDecoder(r.Body).Decode(&payload) != nil {
			w.WriteHeader(http.StatusTeapot)
			w.Write([]byte("Invalid payload"))
			return
		}
		if len(payload.Products) == 0 {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte("No products in payload"))
			return
		}

		orderID := uuid.New()
		err := upsertOrder(ctx, conn, &Order{orderID.String(), "new", payload.Products})
		if err != nil {
			log.Fatal(err.Error())
		}
		sendOrderRequest(kcl, orderID, payload.Products, ctx)

		w.WriteHeader(http.StatusOK)
		w.Write([]byte(orderID.String()))
	})

	port := 80
	log.Printf("Starting server on port %d", port)
	if err := http.ListenAndServe(fmt.Sprintf(":%d", port), nil); err != nil {
		log.Fatalf("Error starting server: %v", err)
	}
}
