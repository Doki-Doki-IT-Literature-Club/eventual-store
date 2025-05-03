package main

import (
	"context"
	"encoding/json"
	"log"
	"math/rand"
	"time"

	"github.com/Doki-Doki-IT-Literature-Club/sops/shared"
	"github.com/jackc/pgx/v5"
	"github.com/twmb/franz-go/pkg/kgo"
)

const kafkaAddress = "kafka:9092"
const orderRequestTopic = "order-request"
const orderRequestResultTopic = "order-request-result"
const orderRequestResultOutboxEventType = "OrderRequestResult"

type OrderRequest struct {
	OrderID  string         `json:"order_id"`
	Products map[string]int `json:"products"`
}

type OrderRequestResult struct {
	OrderID       string `json:"order_id"`
	RequestStatus string `json:"request_status"`
	Reason        string `json:"reason"`
}

func main() {
	log.Printf("Starting inventory service")
	ctx := context.Background()

	ensureDBExists(dbName)
	conn, err := connectToDB()
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer conn.Close(ctx)

	initDB(conn)

	kcl, err := kgo.NewClient(
		kgo.SeedBrokers(kafkaAddress),
		kgo.AllowAutoTopicCreation(),
		kgo.ConsumerGroup("inventory-group"),
		kgo.ConsumeTopics(orderRequestTopic),
	)

	if err != nil {
		log.Fatalf("Error creating Kafka client: %v", err)
	}

	defer kcl.Close()

	go consume(ctx, kcl, conn)

	pollInterval := 5 * time.Second
	shared.ConsumeOutbox(ctx, conn, kcl, pollInterval, map[string]string{orderRequestResultOutboxEventType: orderRequestResultTopic})
}

func processOrderRequest(orderRequest *OrderRequest) *OrderRequestResult {
	status := "accepted"
	reason := "sufficient stock"
	if n := rand.Intn(2); n == 0 {
		status = "rejected"
		reason = "insufficient stock"
	}

	return &OrderRequestResult{
		OrderID:       orderRequest.OrderID,
		RequestStatus: status,
		Reason:        reason,
	}

}

func consume(ctx context.Context, kcl *kgo.Client, conn *pgx.Conn) {
	for {
		events := kcl.PollFetches(ctx)
		if err := events.Err(); err != nil {
			log.Printf("Error fetching events: %v", err)
			continue
		}

		events.EachRecord(func(receivedRecord *kgo.Record) {
			log.Printf("Processing")

			if receivedRecord.Topic == orderRequestTopic {
				handleOrderRequestTopicEvent(ctx, kcl, conn, receivedRecord)
			} else {
				log.Printf("Unknown topic: %s", receivedRecord.Topic)
			}
		})
	}
}

func handleOrderRequestTopicEvent(ctx context.Context, kcl *kgo.Client, conn *pgx.Conn, receivedRecord *kgo.Record) {
	orderRequest := &OrderRequest{}
	if err := json.Unmarshal(receivedRecord.Value, orderRequest); err != nil {
		log.Printf("Error unmarshalling record: %v", err)
		return
	}

	tx, err := conn.Begin(ctx)
	if err != nil {
		if ctx.Err() != nil {
			log.Printf("Context error: %v", ctx.Err())
			return
		}
		log.Printf("Error starting transaction: %v", err)
		return
	}
	defer tx.Rollback(context.Background())

	orderRequestResult := processOrderRequest(orderRequest)

	orderRequestResultBytes, err := json.Marshal(orderRequestResult)
	if err != nil {
		log.Printf("Error marshalling record: %v", err)
		return
	}

	err = shared.InsertOutboxEvent(ctx, tx, "Order", orderRequest.OrderID, orderRequestResultOutboxEventType, orderRequestResultBytes)
	if err != nil {
		log.Printf("Error inserting into outbox: %v", err)
		return
	}

	if err := tx.Commit(ctx); err != nil {
		log.Printf("Error committing transaction: %v", err)
		return
	}

	log.Printf("Transaction committed for OrderID %s", orderRequest.OrderID)
}
