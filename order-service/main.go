package main

import (
	"context"
	"encoding/json"
	"log"

	"fmt"
	"net/http"

	"github.com/Doki-Doki-IT-Literature-Club/sops/order-service/pkg/exmpl"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/twmb/franz-go/pkg/kgo"
)

const (
	base2Address              = "http://base-2-base-2-1:8002"
	kafkaAddress              = "kafka:9092"
	dbConnString              = "postgres://user:password@postgres:5432/"
	dbName                    = "orders"
	orderShippingRequestTopic = "order-shipping-request"
	orderShippingStatusTopic  = "order-shipping-status"
	orderRequestTopic         = "order-request"
	orderRequestResultTopic   = "order-request-result"
)

type OrderShippingStatus struct {
	OrderID string `json:"order_id"`
	Status  string `json:"status"`
}

type OrderRequest struct {
	OrderID  string         `json:"order_id"`
	Products map[string]int `json:"products"`
}

type OrderRequestResponse struct {
	OrderID       string `json:"order_id"`
	RequestStatus string `json:"request_status"`
}
type CreateOrderPayload struct {
	Products map[string]int `json:"products"`
}

type OrderShippingRequest struct {
	OrderID string `json:"order_id"`
}

func sendOrderRequest(kcl *kgo.Client, orderID uuid.UUID, products map[string]int, ctx context.Context) {
	orderRequestPayload := &OrderRequest{
		OrderID:  orderID.String(),
		Products: products,
	}

	orderRequestRecord, err := json.Marshal(orderRequestPayload)
	if err != nil {
		log.Printf("Error marshalling order request: %v", err)
		return
	}

	record := &kgo.Record{
		Topic: orderRequestTopic,
		Value: orderRequestRecord,
	}

	kcl.Produce(ctx, record, func(_ *kgo.Record, err error) {
		if err != nil {
			log.Printf("record had a produce error: %v\n", err)
		}

		log.Printf("Produced record to topic %s", orderRequestTopic)
	})
}

func sendOrderShippingRequest(kcl *kgo.Client, orderID string, ctx context.Context) {
	orderShippingRequestPayload := OrderShippingRequest{orderID}

	orderShippingRequestRecord, err := json.Marshal(orderShippingRequestPayload)
	if err != nil {
		log.Printf("Error marshalling order request: %v", err)
		return
	}

	record := &kgo.Record{
		Topic: orderShippingRequestTopic,
		Value: orderShippingRequestRecord,
	}

	kcl.Produce(ctx, record, func(_ *kgo.Record, err error) {
		if err != nil {
			log.Printf("record had a produce error: %v\n", err)
		}

		log.Printf("Produced record to topic %s", orderShippingRequestTopic)
	})
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
		sendOrderRequest(kcl, orderID, payload.Products, ctx)

		w.WriteHeader(http.StatusOK)
		w.Write([]byte(orderID.String()))
	})

	port := 8002
	log.Printf("Starting server on port %d", port)
	if err := http.ListenAndServe(fmt.Sprintf(":%d", port), nil); err != nil {
		log.Fatalf("Server error: %v", err)

		if err != nil {
			panic(err)
		}
	}
}

func consume(kcl *kgo.Client, conn *pgx.Conn, ctx context.Context) {
	log.Printf("Start consuming events...")
	for {
		events := kcl.PollFetches(ctx)
		if err := events.Err(); err != nil {
			log.Printf("Error fetching events: %v", err)
			continue
		}

		events.EachRecord(func(r *kgo.Record) {
			log.Printf("New record")
			if r.Topic == orderRequestResultTopic {
				log.Printf("Processing record from topic %s", r.Topic)

				orderRequestResponse := &OrderRequestResponse{}
				if err := json.Unmarshal(r.Value, orderRequestResponse); err != nil {
					log.Printf("Error unmarshalling record: %v", err)
					return
				}

				err := updateOrder(conn, orderRequestResponse.OrderID, orderRequestResponse.RequestStatus)
				if err != nil {
					log.Printf(err.Error())
				}
				if orderRequestResponse.RequestStatus == "accepted" {
					sendOrderShippingRequest(kcl, orderRequestResponse.OrderID, ctx)
					log.Printf("Sent shipping request")
				}
			} else if r.Topic == orderShippingStatusTopic {
				log.Printf("Processing record from topic %s", r.Topic)

				orderShippingStatus := &OrderShippingStatus{}
				if err := json.Unmarshal(r.Value, orderShippingStatus); err != nil {
					log.Printf("Error unmarshalling record: %v", err)
					return
				}

				err := updateOrder(conn, orderShippingStatus.OrderID, orderShippingStatus.Status)
				if err != nil {
					log.Printf(err.Error())
				}

			} else {
				log.Printf("Record from topic %s, skipping for now...", r.Topic)
			}
		})
	}
}

func connectToDB() (*pgx.Conn, error) {
	conn, err := pgx.Connect(context.Background(), dbConnString+"/"+dbName)
	if err != nil {
		return nil, fmt.Errorf("unable to connect to database: %v", err)
	}
	return conn, nil
}

func initDB(conn *pgx.Conn) error {
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

func updateOrder(conn *pgx.Conn, orderID string, status string) error {
	log.Print("Starting updating")
	query := "UPDATE orders set status=$1 WHERE id=$2"
	_, err := conn.Exec(context.Background(), query, status, orderID)
	if err != nil {
		return fmt.Errorf("unable to update: %v", err)
	}
	return nil
}

func main() {
	log.Printf("Starting order service")

	ctx := context.Background()
	kcl, err := kgo.NewClient(
		kgo.SeedBrokers(kafkaAddress),
		kgo.AllowAutoTopicCreation(),
		kgo.ConsumerGroup("order-group"),
		kgo.ConsumeTopics(
			orderRequestResultTopic,
			orderShippingStatusTopic,
		),
	)

	if err != nil {
		log.Fatalf("Error creating Kafka client: %v", err)
	}

	ensureDBExists(dbConnString, dbName)

	conn, err := connectToDB()
	if err != nil {
		log.Fatalf("Error connecting to database: %v", err)
	}

	defer conn.Close(ctx)

	if err := initDB(conn); err != nil {
		log.Fatalf("Error creating test table: %v", err)
	}

	defer kcl.Close()
	go consume(kcl, conn, ctx)
	httpServer(conn, kcl, ctx)
}

func ensureDBExists(dbConnString string, dbName string) {
	initialDbConnString := dbConnString + "postgres"
	conn, err := pgx.Connect(context.Background(), initialDbConnString)
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
