package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/Doki-Doki-IT-Literature-Club/sops/shared"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/twmb/franz-go/pkg/kgo"
)

const (
	kafkaAddress               = "kafka:9092"
	dbName                     = "orders"
	orderUpdateOutboxEventType = "OrderUpdate"
)

type CreateOrderPayload struct {
	Products map[string]int `json:"products"`
}

type Order struct {
	ID       string         `json:"id"`
	Status   string         `json:"status"`
	Products map[string]int `json:"products"`
}

func main() {
	log.Printf("Starting order service")

	ctx := context.Background()
	kcl, err := kgo.NewClient(
		kgo.SeedBrokers(kafkaAddress),
		kgo.AllowAutoTopicCreation(),
		kgo.ConsumerGroup("order-group"),
		kgo.ConsumeTopics(
			shared.OrderRequestResultTopic,
			shared.OrderShippingStatusTopic,
		),
	)

	if err != nil {
		log.Fatalf("Error creating Kafka client: %v", err)
	}

	err = shared.EnsureDBExists(ctx, dbName)
	if err != nil {
		log.Fatalf("Error ensuring database exists: %v", err)
	}

	conn, err := shared.ConnectToDB(ctx, dbName)
	if err != nil {
		log.Fatalf("Error connecting to database: %v", err)
	}

	defer conn.Close(ctx)

	if err := initDB(ctx, conn); err != nil {
		log.Fatalf("Error creating test table: %v", err)
	}

	defer kcl.Close()
	go consume(kcl, conn, ctx)
	go shared.ConsumeOutbox(ctx, conn, kcl, time.Second, map[string]string{orderUpdateOutboxEventType: shared.OrderStateTopic})
	httpServer(conn, kcl, ctx)
}

func httpServer(conn *pgx.Conn, kcl *kgo.Client, ctx context.Context) {
	http.HandleFunc("POST /orders", func(w http.ResponseWriter, r *http.Request) {
		log.Printf("Received request: %s %s", r.Method, r.URL.Path)
		payload := CreateOrderPayload{}
		if json.NewDecoder(r.Body).Decode(&payload) != nil {
			w.WriteHeader(http.StatusTeapot)
			w.Write([]byte("Invalid payload"))
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

func sendOrderRequest(kcl *kgo.Client, orderID uuid.UUID, products map[string]int, ctx context.Context) {
	orderRequestPayload := &shared.OrderRequestEvent{
		OrderID:  orderID.String(),
		Products: products,
	}

	orderRequestRecord, err := json.Marshal(orderRequestPayload)
	if err != nil {
		log.Printf("Error marshalling order request: %v", err)
		return
	}

	record := &kgo.Record{
		Topic: shared.OrderRequestTopic,
		Value: orderRequestRecord,
	}

	kcl.Produce(ctx, record, func(_ *kgo.Record, err error) {
		if err != nil {
			log.Printf("record had a produce error: %v\n", err)
		}

		log.Printf("Produced record to topic %s", shared.OrderRequestTopic)
	})
}

func sendOrderShippingRequest(kcl *kgo.Client, orderID string, ctx context.Context) {
	orderShippingRequestPayload := shared.OrderShippingRequestEvent{OrderID: orderID}

	orderShippingRequestRecord, err := json.Marshal(orderShippingRequestPayload)
	if err != nil {
		log.Printf("Error marshalling order request: %v", err)
		return
	}

	record := &kgo.Record{
		Topic: shared.OrderShippingRequestTopic,
		Value: orderShippingRequestRecord,
	}

	kcl.Produce(ctx, record, func(_ *kgo.Record, err error) {
		if err != nil {
			log.Printf("record had a produce error: %v\n", err)
		}

		log.Printf("Produced record to topic %s", shared.OrderShippingRequestTopic)
	})
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
			if r.Topic == shared.OrderRequestResultTopic {
				log.Printf("Processing record from topic %s", r.Topic)

				orderRequestResponse := &shared.OrderRequestResultEvent{}
				if err := json.Unmarshal(r.Value, orderRequestResponse); err != nil {
					log.Printf("Error unmarshalling record: %v", err)
					return
				}

				// TODO: get order fist, pass products
				err := upsertOrder(ctx, conn, &Order{ID: orderRequestResponse.OrderID, Status: orderRequestResponse.RequestStatus})
				if err != nil {
					log.Print(err.Error())
				}
				if orderRequestResponse.RequestStatus == "accepted" {
					sendOrderShippingRequest(kcl, orderRequestResponse.OrderID, ctx)
					log.Printf("Sent shipping request")
				}
			} else if r.Topic == shared.OrderShippingStatusTopic {
				log.Printf("Processing record from topic %s", r.Topic)

				orderShippingStatus := &shared.OrderShippingStatusEvent{}
				if err := json.Unmarshal(r.Value, orderShippingStatus); err != nil {
					log.Printf("Error unmarshalling record: %v", err)
					return
				}

				// TODO: get order fist, pass products
				err := upsertOrder(ctx, conn, &Order{ID: orderShippingStatus.OrderID, Status: orderShippingStatus.Status})
				if err != nil {
					log.Print(err.Error())
				}

			} else {
				log.Printf("Record from topic %s, skipping for now...", r.Topic)
			}
		})
	}
}
