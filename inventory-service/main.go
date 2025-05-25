package main

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"github.com/Doki-Doki-IT-Literature-Club/sops/shared"
	"github.com/jackc/pgx/v5"
	"github.com/twmb/franz-go/pkg/kgo"
)

func main() {
	log.Printf("Starting inventory service")
	ctx := context.Background()

	err := shared.EnsureDBExists(ctx, dbName)
	if err != nil {
		log.Fatalf("Failed to ensure database exists: %v", err)
	}

	conn, err := shared.ConnectToDB(ctx, dbName)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer conn.Close(ctx)

	err = initDB(ctx, conn)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}

	err = fillDB(conn, ctx)
	if err != nil {
		log.Fatalf("Failed to fill database: %v", err)
	}

	kcl, err := kgo.NewClient(
		kgo.SeedBrokers(shared.KafkaAddress),
		kgo.AllowAutoTopicCreation(),
		kgo.ConsumerGroup("inventory-group"),
		kgo.ConsumeTopics(shared.OrderRequestTopic),
	)

	if err != nil {
		log.Fatalf("Error creating Kafka client: %v", err)
	}

	defer kcl.Close()

	go consume(ctx, kcl, conn)

	pollInterval := 5 * time.Second
	shared.ConsumeOutbox(ctx, conn, kcl, pollInterval, map[string]string{
		orderRequestResultOutboxEventType: shared.OrderRequestResultTopic,
		inventoryItemOutboxEventType:      shared.InventoryStateTopic,
	})
}

// potential race condition
func processOrderRequest(con *pgx.Conn, orderRequest *shared.OrderRequestEvent) *shared.OrderRequestResultEvent {
	log.Printf("Processing order request: %v, %v\n", orderRequest.OrderID, orderRequest.Products)

	orderedItemsIds := make([]string, len(orderRequest.Products))
	for id := range orderRequest.Products {
		orderedItemsIds = append(orderedItemsIds, id)
	}

	items, err := getInventoryItems(con, context.Background(), orderedItemsIds)
	if err != nil {
		log.Printf("Error getting inventory items: %v", err)
		return &shared.OrderRequestResultEvent{
			OrderID:       orderRequest.OrderID,
			RequestStatus: "rejected",
			Reason:        "internal error",
		}
	}

	for i := range len(items) {
		item := &items[i]
		if item.Amount < orderRequest.Products[item.ID] {
			return &shared.OrderRequestResultEvent{
				OrderID:       orderRequest.OrderID,
				RequestStatus: "rejected",
				Reason:        "insufficient stock",
			}
		}

		item.Amount -= orderRequest.Products[item.ID]
	}

	// TODO: should be single transaction
	err = updateInventoryItems(con, context.Background(), items)
	if err != nil {
		log.Printf("Error updating inventory items: %v", err)
		return &shared.OrderRequestResultEvent{
			OrderID:       orderRequest.OrderID,
			RequestStatus: "rejected",
			Reason:        "internal error",
		}
	}
	log.Printf("Updated inventory items: %v\n", items)

	return &shared.OrderRequestResultEvent{
		OrderID:       orderRequest.OrderID,
		RequestStatus: "success",
		Reason:        "sufficient stock",
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

			if receivedRecord.Topic == shared.OrderRequestTopic {
				handleOrderRequestTopicEvent(ctx, kcl, conn, receivedRecord)
			} else {
				log.Printf("Unknown topic: %s", receivedRecord.Topic)
			}
		})
	}
}

func handleOrderRequestTopicEvent(ctx context.Context, kcl *kgo.Client, conn *pgx.Conn, receivedRecord *kgo.Record) {
	orderRequest := &shared.OrderRequestEvent{}
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

	// TODO: single transaction
	orderRequestResult := processOrderRequest(conn, orderRequest)

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
