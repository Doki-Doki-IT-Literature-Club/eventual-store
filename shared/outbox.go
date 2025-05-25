package shared

import (
	"context"
	"fmt"
	"log"
	"slices"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/twmb/franz-go/pkg/kgo"
)

type outboxEvent struct {
	ID          string
	EventType   string
	AggregateID string
	Payload     []byte
}

func InsertOutboxEvent(
	ctx context.Context,
	tx pgx.Tx,
	aggType string,
	aggID string,
	eventType string,
	data []byte,
) error {
	eventID, err := uuid.NewRandom()
	if err != nil {
		return err
	}

	outboxInsertSQL := `
	INSERT INTO outbox_events (id, aggregate_type, aggregate_id, event_type, payload, status)
	VALUES ($1, $2, $3, $4, $5, 'PENDING')`

	_, err = tx.Exec(ctx, outboxInsertSQL,
		eventID, aggType, aggID, eventType, data,
	)
	if err != nil {
		return fmt.Errorf("unable to add outbox event: %v", err)
	}

	log.Printf("Added outbox event %s", eventID.String())

	return nil
}

func ConsumeOutbox(ctx context.Context, conn *pgx.Conn, kcl *kgo.Client, pollInterval time.Duration, eventTypeToTopicMap map[string]string) {
	log.Println("Outbox consumer started.")
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			err := processOutboxBatch(ctx, conn, kcl, eventTypeToTopicMap)
			if err != nil {
				log.Printf("Error processing outbox batch: %v", err)
			}
		}
	}
}

func processOutboxBatch(ctx context.Context, conn *pgx.Conn, kcl *kgo.Client, eventTypeToTopicName map[string]string) error {
	selectSQL := `
	SELECT id, payload, event_type, aggregate_id
	FROM outbox_events
	WHERE status = 'PENDING'
	ORDER BY created_at
	`
	rows, err := conn.Query(ctx, selectSQL)
	if err != nil {
		return fmt.Errorf("error querying outbox: %w", err)
	}
	defer rows.Close()

	var eventsToPublish []outboxEvent
	for rows.Next() {
		var event outboxEvent
		if err := rows.Scan(&event.ID, &event.Payload, &event.EventType, &event.AggregateID); err != nil {
			return fmt.Errorf("error scanning row: %w", err)
		}
		eventsToPublish = append(eventsToPublish, event)
	}

	if err := rows.Err(); err != nil {
		return fmt.Errorf("error iterating rows: %w", err)
	}

	if len(eventsToPublish) == 0 {
		return nil
	}

	publishedEventsIds := make([]string, 0)
	failedEventsIds := make([]string, 0)

	recordsToPublish := make([]*kgo.Record, len(eventsToPublish))
	for i, event := range eventsToPublish {
		topic, ok := eventTypeToTopicName[event.EventType]
		if !ok {
			log.Printf("Unknown event type: %s", event.EventType)
			continue
		}

		record := &kgo.Record{
			Key:   []byte(event.AggregateID),
			Topic: topic,
			Value: event.Payload,
			Headers: []kgo.RecordHeader{
				{Key: "outbox_id", Value: []byte(event.ID)},
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
