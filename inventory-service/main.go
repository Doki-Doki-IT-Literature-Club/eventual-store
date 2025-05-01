package main

import (
	"context"
	"encoding/json"
	"log"
	"math/rand"

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
}

func main() {
	log.Printf("Starting inventory service")

	ctx := context.Background()
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

	consume(kcl, ctx)
}

func processOrderRequest(orderRequest *OrderRequest) *OrderRequestResult {
	status := "accepted"
	if n := rand.Intn(2); n == 0 {
		status = "rejected"
	}

	return &OrderRequestResult{
		OrderID:       orderRequest.OrderID,
		RequestStatus: status,
	}

}

func consume(kcl *kgo.Client, ctx context.Context) {
	for {
		events := kcl.PollFetches(ctx)
		if err := events.Err(); err != nil {
			log.Printf("Error fetching events: %v", err)
			continue
		}

		events.EachRecord(func(r *kgo.Record) {
			log.Printf("Processing")

			orderRequest := &OrderRequest{}
			if err := json.Unmarshal(r.Value, orderRequest); err != nil {
				log.Printf("Error unmarshalling record: %v", err)
				return
			}

			orderRequestResult := processOrderRequest(orderRequest)

			orderRequestResultBytes, err := json.Marshal(orderRequestResult)
			if err != nil {
				log.Printf("Error marshalling record: %v", err)
				return
			}

			record := &kgo.Record{
				Topic: orderRequestResultTopic,
				Value: orderRequestResultBytes,
			}

			kcl.Produce(ctx, record, func(_ *kgo.Record, err error) {
				if err != nil {
					log.Printf("record had a produce error: %v\n", err)
				}

				log.Printf("Produced record to topic %s", orderRequestTopic)
			})
		})
	}
}
