package main

import (
	"context"
	"encoding/json"
	"log"
	"time"

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

func main() {
	log.Printf("Starting base-1 service")

	kcl, err := kgo.NewClient(
		kgo.SeedBrokers(kafkaAddress),
		kgo.AllowAutoTopicCreation(),
		kgo.ConsumerGroup("shipping-group"),
		kgo.ConsumeTopics(orderShippingRequestTopic),
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

			orderShippingRequest := &OrderShippingRequest{}
			if err := json.Unmarshal(r.Value, orderShippingRequest); err != nil {
				log.Printf("Error unmarshalling record: %v", err)
				return
			}
			time.Sleep(time.Second * 5)

			orderShippingStatusInDelivery := &OrderShippingStatus{
				OrderID: orderShippingRequest.OrderID,
				Status:  "In Delivery",
			}

			orderShippingStatusDeliveryBytes, err := json.Marshal(orderShippingStatusInDelivery)
			if err != nil {
				log.Printf("Error marshalling order shipping status: %v", err)
				return
			}

			record := &kgo.Record{
				Topic: orderShippingStatusTopic,
				Value: orderShippingStatusDeliveryBytes,
			}

			kcl.Produce(ctx, record, func(_ *kgo.Record, err error) {
				if err != nil {
					log.Printf("record had a produce error: %v\n", err)
				}

				log.Printf("Produced record to topic %s", orderShippingStatusTopic)
			})

			time.Sleep(time.Second * 10)

			orderShippingStatusDelivered := &OrderShippingStatus{
				OrderID: orderShippingRequest.OrderID,
				Status:  "Delivered",
			}

			orderShippingStatusDeliveredBytes, err := json.Marshal(orderShippingStatusDelivered)
			if err != nil {
				log.Printf("Error marshalling order shipping status: %v", err)
				return
			}

			record = &kgo.Record{
				Topic: orderShippingStatusTopic,
				Value: orderShippingStatusDeliveredBytes,
			}

			kcl.Produce(ctx, record, func(_ *kgo.Record, err error) {
				if err != nil {
					log.Printf("record had a produce error: %v\n", err)
				}

				log.Printf("Produced record to topic %s", orderShippingStatusTopic)
			})
		})
	}
}
