package main

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"github.com/Doki-Doki-IT-Literature-Club/sops/shared"

	"github.com/twmb/franz-go/pkg/kgo"
)

func main() {
	log.Printf("Starting base-1 service")

	kcl, err := kgo.NewClient(
		kgo.SeedBrokers(shared.KafkaAddress),
		kgo.AllowAutoTopicCreation(),
		kgo.ConsumerGroup("shipping-group"),
		kgo.ConsumeTopics(shared.OrderShippingRequestTopic),
	)

	if err != nil {
		log.Fatalf("Error creating Kafka client: %v", err)
	}

	defer kcl.Close()

	ctx := context.Background()

	for {
		events := kcl.PollFetches(ctx)
		if err := events.Err(); err != nil {
			log.Printf("Error fetching events: %v", err)
			continue
		}

		events.EachRecord(func(r *kgo.Record) {
			log.Printf("Processing")

			orderShippingRequest := &shared.OrderShippingRequestEvent{}
			if err := json.Unmarshal(r.Value, orderShippingRequest); err != nil {
				log.Printf("Error unmarshalling record: %v", err)
				return
			}
			time.Sleep(time.Second * 5)

			orderShippingStatusInDelivery := &shared.OrderShippingStatusEvent{
				OrderID: orderShippingRequest.OrderID,
				Status:  "In Delivery",
			}

			orderShippingStatusDeliveryBytes, err := json.Marshal(orderShippingStatusInDelivery)
			if err != nil {
				log.Printf("Error marshalling order shipping status: %v", err)
				return
			}

			record := &kgo.Record{
				Topic: shared.OrderShippingStatusTopic,
				Value: orderShippingStatusDeliveryBytes,
			}

			kcl.Produce(ctx, record, func(_ *kgo.Record, err error) {
				if err != nil {
					log.Printf("record had a produce error: %v\n", err)
				}

				log.Printf("Produced record to topic %s", shared.OrderShippingStatusTopic)
			})

			time.Sleep(time.Second * 10)

			orderShippingStatusDelivered := &shared.OrderShippingStatusEvent{
				OrderID: orderShippingRequest.OrderID,
				Status:  "Delivered",
			}

			orderShippingStatusDeliveredBytes, err := json.Marshal(orderShippingStatusDelivered)
			if err != nil {
				log.Printf("Error marshalling order shipping status: %v", err)
				return
			}

			record = &kgo.Record{
				Topic: shared.OrderShippingStatusTopic,
				Value: orderShippingStatusDeliveredBytes,
			}

			kcl.Produce(ctx, record, func(_ *kgo.Record, err error) {
				if err != nil {
					log.Printf("record had a produce error: %v\n", err)
				}

				log.Printf("Produced record to topic %s", shared.OrderShippingStatusTopic)
			})
		})
	}
}
