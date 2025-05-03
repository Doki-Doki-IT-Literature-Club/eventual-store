package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"slices"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/twmb/franz-go/pkg/kgo"
)

const kafkaAddress = "kafka:9092"
const orderRequestTopic = "order-request"
const orderRequestResultTopic = "order-request-result"

type OrderRequest struct {
	OrderID  string         `json:"order_id"`
	Products map[string]int `json:"products"`
}

type OrderRequestResult struct {
	OrderID       string `json:"order_id"`
	RequestStatus string `json:"request_status"`
	Reason        string `json:"reason"`
}

type OutboxEvent struct {
	ID        uuid.UUID
	EventType string
	Payload   []byte
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
	consumeOutbox(ctx, conn, kcl, pollInterval)
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

	eventID, err := uuid.NewRandom()
	if err != nil {
		log.Printf("Error generating UUID: %v", err)
		return
	}

	outboxInsertSQL := `
	INSERT INTO outbox_events (id, aggregate_type, aggregate_id, event_type, payload, status)
	VALUES ($1, $2, $3, $4, $5, 'PENDING')`
	_, err = tx.Exec(ctx, outboxInsertSQL,
		eventID, "Order", orderRequest.OrderID, "OrderRequestResult", orderRequestResultBytes,
	)
	if err != nil {
		log.Printf("Error inserting into outbox: %v", err)
		return
	}
	log.Printf("Inserted event %s into outbox for OrderID %s", eventID, orderRequest.OrderID)

	if err := tx.Commit(ctx); err != nil {
		log.Printf("Error committing transaction: %v", err)
		return
	}

	log.Printf("Transaction committed for OrderID %s", orderRequest.OrderID)
}

func processOutboxBatch(ctx context.Context, conn *pgx.Conn, kcl *kgo.Client) error {
	selectSQL := `
	SELECT id, payload, event_type
	FROM outbox_events
	WHERE status = 'PENDING'
	ORDER BY created_at
	`
	rows, err := conn.Query(ctx, selectSQL)
	if err != nil {
		return fmt.Errorf("error querying outbox: %w", err)
	}
	defer rows.Close()

	var eventsToPublish []OutboxEvent
	for rows.Next() {
		var event OutboxEvent
		if err := rows.Scan(&event.ID, &event.Payload, &event.EventType); err != nil {
			return fmt.Errorf("error scanning row: %w", err)
		}
		eventsToPublish = append(eventsToPublish, event)
	}

	if err := rows.Err(); err != nil {
		return fmt.Errorf("error iterating rows: %w", err)
	}

	if len(eventsToPublish) == 0 {
		log.Println("No pending events to publish")
		return nil
	}

	publishedEventsIds := make([]string, 0)
	failedEventsIds := make([]string, 0)

	recordsToPublish := make([]*kgo.Record, len(eventsToPublish))
	for i, event := range eventsToPublish {
		topic := ""
		switch event.EventType {
		case "OrderRequestResult":
			topic = orderRequestResultTopic
		default:
			log.Printf("Unknown event type: %s", event.EventType)
			continue
		}

		record := &kgo.Record{
			Topic: topic,
			Value: event.Payload,
			Headers: []kgo.RecordHeader{
				{Key: "outbox_id", Value: []byte(event.ID.String())},
			},
		}
		recordsToPublish[i] = record
	}

	// order is not important
	results := kcl.ProduceSync(ctx, recordsToPublish...)
	for _, result := range results {
		outboxIdHeaderIndex := slices.IndexFunc(result.Record.Headers, func(h kgo.RecordHeader) bool {
			return string(h.Key) == "outbox_id"
		})

		if outboxIdHeaderIndex == -1 {
			log.Printf("No outbox_id header found in produced record")
			continue
		}

		if result.Err != nil {
			failedEventsIds = append(failedEventsIds, string(result.Record.Headers[outboxIdHeaderIndex].Value))
			log.Printf("Error producing record: %v", result.Err)
			continue
		}

		outboxId := string(result.Record.Headers[outboxIdHeaderIndex].Value)
		publishedEventsIds = append(publishedEventsIds, outboxId)
	}

	updatePubloshedSQL := `
	UPDATE outbox_events
	SET status = 'PUBLISHED', published_at = NOW()
	WHERE id = ANY($1)
	`

	updateFailedSQL := `
	UPDATE outbox_events
	SET status = 'FAILED', published_at = NOW()
	WHERE id = ANY($1)
	`

	tx, err := conn.Begin(ctx)
	if err != nil {
		return fmt.Errorf("error starting transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	if len(publishedEventsIds) > 0 {
		_, err = tx.Exec(ctx, updatePubloshedSQL, publishedEventsIds)
		if err != nil {
			return fmt.Errorf("error updating published events: %w", err)
		}
		log.Printf("Updated %d events to PUBLISHED", len(publishedEventsIds))
	}

	if len(failedEventsIds) > 0 {
		_, err = tx.Exec(ctx, updateFailedSQL, failedEventsIds)
		if err != nil {
			return fmt.Errorf("error updating failed events: %w", err)
		}
		log.Printf("Updated %d events to FAILED", len(failedEventsIds))
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("error committing transaction: %w", err)
	}

	log.Printf("Transaction committed")

	return nil

}

func consumeOutbox(ctx context.Context, conn *pgx.Conn, kcl *kgo.Client, pollInterval time.Duration) {
	log.Println("Outbox consumer started.")
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			err := processOutboxBatch(ctx, conn, kcl)
			if err != nil {
				log.Printf("Error processing outbox batch: %v", err)
			}
		}
	}
}
