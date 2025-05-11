package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/Doki-Doki-IT-Literature-Club/sops/shared"
	"github.com/jackc/pgx/v5"
	"github.com/twmb/franz-go/pkg/kgo"
)

func initKafkaClient() (*kgo.Client, error) {
	kcl, err := kgo.NewClient(
		kgo.SeedBrokers(shared.KafkaAddress),
		kgo.AllowAutoTopicCreation(),
		kgo.ConsumerGroup("showcase-group"),
		kgo.ConsumeTopics(
			shared.OrderStateTopic,
			shared.InventoryStateTopic,
		),
	)
	if err != nil {
		return nil, err
	}

	return kcl, nil
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
			log.Printf("Processing record from topic %s", r.Topic)

			switch r.Topic {
			case shared.OrderStateTopic:
				if err := consumeOrderStateRecord(ctx, r, conn); err != nil {
					log.Printf("Error consuming record: %v", err)
				}
			case shared.InventoryStateTopic:
				if err := consumeInventoryStateRecord(ctx, r, conn); err != nil {
					log.Printf("Error consuming record: %v", err)
				}
			default:
				log.Printf("Unkown topic %s", r.Topic)
			}
		})
	}
}

func consumeInventoryStateRecord(ctx context.Context, record *kgo.Record, conn *pgx.Conn) error {
	InventoryStateEvent := &shared.InventoryStateEvent{}
	if err := json.Unmarshal(record.Value, InventoryStateEvent); err != nil {
		return fmt.Errorf("error unmarshalling record: %v", err)
	}

	inventory := &Inventory{
		ID:     InventoryStateEvent.ID,
		Amount: InventoryStateEvent.Amount,
	}

	err := updateInventory(conn, ctx, *inventory)
	if err != nil {
		return fmt.Errorf("error updating inventory: %v", err)
	}
	return nil
}

func consumeOrderStateRecord(ctx context.Context, record *kgo.Record, conn *pgx.Conn) error {
	orderStateEvent := &shared.OrderStateEvent{}
	if err := json.Unmarshal(record.Value, orderStateEvent); err != nil {
		return fmt.Errorf("error unmarshalling record: %v", err)
	}

	order := &Order{
		ID:       orderStateEvent.ID,
		Products: orderStateEvent.Products,
		Status:   orderStateEvent.Status,
	}

	err := updateOrder(conn, ctx, *order)
	if err != nil {
		return fmt.Errorf("error updating order: %v", err)
	}

	return nil
}
