package main

import (
	"context"
	"encoding/json"
	"log"
	// "time"

	"base/pkg/exmpl"
	"fmt"
	"net/http"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/twmb/franz-go/pkg/kgo"
)

const base2Address = "http://base-2-base-2-1:8002"
const kafkaAddress = "kafka:9092"
const dbConnString = "postgres://user:password@postgres:5432/mydb"
const orderShippingRequestTopic = "order-shipping-request"
const orderShippingStatusTopic = "order-shipping-status"

type OrderShippingRequest struct {
	OrderID string `json:"order_id"`
}

type OrderShippingStatus struct {
	OrderID string `json:"order_id"`
	Status  string `json:"status"`
}

type CreateOrderPayload struct {
	Products map[string]int `json:"products"`
}

func httpServer(conn *pgx.Conn, kcl *kgo.Client, ctx context.Context) {
	http.HandleFunc("POST /orders", func(w http.ResponseWriter, r *http.Request) {
		log.Printf("Received request: %s %s", r.Method, r.URL.Path)
		payload := CreateOrderPayload{}
		if json.NewDecoder(r.Body).Decode(&payload) != nil {
			w.WriteHeader(http.StatusTeapot)
			w.Write([]byte(exmpl.A))
			return
		}
		orderID := uuid.New()
		insertOrder(conn, orderID, "new", payload.Products)

		w.WriteHeader(http.StatusOK)
		w.Write([]byte(exmpl.A))
	})

	// http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
	// 	log.Printf("Received request: %s %s", r.Method, r.URL.Path)
	// 	w.WriteHeader(http.StatusOK)
	// 	w.Write([]byte(exmpl.A))
	// })
	port := 8002
	log.Printf("Starting server on port %d", port)
	if err := http.ListenAndServe(fmt.Sprintf(":%d", port), nil); err != nil {
		log.Fatalf("Server error: %v", err)

		if err != nil {
			panic(err)
		}
	}
}

// func consume(kcl *kgo.Client, ctx context.Context) {
// 	for {
// 		events := kcl.PollFetches(ctx)
// 		if err := events.Err(); err != nil {
// 			log.Printf("Error fetching events: %v", err)
// 			continue
// 		}
//
// 		events.EachRecord(func(r *kgo.Record) {
// 			log.Printf("Processing")
//
// 			orderShippingRequest := &OrderShippingRequest{}
// 			if err := json.Unmarshal(r.Value, orderShippingRequest); err != nil {
// 				log.Printf("Error unmarshalling record: %v", err)
// 				return
// 			}
// 			time.Sleep(time.Second * 5)
//
// 			orderShippingStatusInDelivery := &OrderShippingStatus{
// 				OrderID: orderShippingRequest.OrderID,
// 				Status:  "In Delivery",
// 			}
//
// 			orderShippingStatusDeliveryBytes, err := json.Marshal(orderShippingStatusInDelivery)
// 			if err != nil {
// 				log.Printf("Error marshalling order shipping status: %v", err)
// 				return
// 			}
//
// 			record := &kgo.Record{
// 				Topic: orderShippingStatusTopic,
// 				Value: orderShippingStatusDeliveryBytes,
// 			}
//
// 			kcl.Produce(ctx, record, func(_ *kgo.Record, err error) {
// 				if err != nil {
// 					log.Printf("record had a produce error: %v\n", err)
// 				}
//
// 				log.Printf("Produced record to topic %s", orderShippingStatusTopic)
// 			})
//
// 			time.Sleep(time.Second * 10)
//
// 			orderShippingStatusDelivered := &OrderShippingStatus{
// 				OrderID: orderShippingRequest.OrderID,
// 				Status:  "Delivered",
// 			}
//
// 			orderShippingStatusDeliveredBytes, err := json.Marshal(orderShippingStatusDelivered)
// 			if err != nil {
// 				log.Printf("Error marshalling order shipping status: %v", err)
// 				return
// 			}
//
// 			record = &kgo.Record{
// 				Topic: orderShippingStatusTopic,
// 				Value: orderShippingStatusDeliveredBytes,
// 			}
//
// 			kcl.Produce(ctx, record, func(_ *kgo.Record, err error) {
// 				if err != nil {
// 					log.Printf("record had a produce error: %v\n", err)
// 				}
//
// 				log.Printf("Produced record to topic %s", orderShippingStatusTopic)
// 			})
// 		})
// 	}
// }

func connectToDB() (*pgx.Conn, error) {
	conn, err := pgx.Connect(context.Background(), dbConnString)
	if err != nil {
		return nil, fmt.Errorf("unable to connect to database: %v", err)
	}
	return conn, nil
}

func createTables(conn *pgx.Conn) error {
	_, err := conn.Exec(context.Background(), "CREATE TABLE IF NOT EXISTS orders (id TEXT PRIMARY KEY, status TEXT, products TEXT)")
	if err != nil {
		return fmt.Errorf("unable to create table: %v", err)
	}
	return nil
}

func insertOrder(conn *pgx.Conn, orderID uuid.UUID, status string, products map[string]int) error {
	log.Print("Starting inserting")
	query := fmt.Sprintf("INSERT INTO orders (id, status, products) VALUES ($1, $2, $3)")
	productsBytes, err := json.Marshal(products)
	_, err = conn.Exec(context.Background(), query, orderID.String(), status, string(productsBytes))
	if err != nil {
		return fmt.Errorf("unable to insert: %v", err)
	}
	return nil
}

func main() {
	log.Printf("Starting order service")

	ctx := context.Background()
	kcl, err := kgo.NewClient(
		kgo.SeedBrokers(kafkaAddress),
		kgo.AllowAutoTopicCreation(),
		kgo.ConsumerGroup("shipping-group"),
		kgo.ConsumeTopics(orderShippingRequestTopic),
	)

	if err != nil {
		log.Fatalf("Error creating Kafka client: %v", err)
	}

	conn, err := connectToDB()
	if err != nil {
		log.Fatalf("Error connecting to database: %v", err)
	}

	defer conn.Close(ctx)
	if err := createTables(conn); err != nil {
		log.Fatalf("Error creating test table: %v", err)
	}

	defer kcl.Close()
	httpServer(conn, kcl, ctx)

	// consume(kcl, ctx)
}
