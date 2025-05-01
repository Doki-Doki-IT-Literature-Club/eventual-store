package main

import (
	"base/pkg/exmpl"
	"context"
	"fmt"
	"log"
	"net/http"

	"github.com/jackc/pgx/v5"
	"github.com/twmb/franz-go/pkg/kgo"
)

var base1Address = "http://base-1:8001"
var kafkaAddress = "kafka:9092"
var dbConnString = "postgres://user:password@postgres:5432/mydb"

func connectToDB() (*pgx.Conn, error) {
	conn, err := pgx.Connect(context.Background(), dbConnString)
	if err != nil {
		return nil, fmt.Errorf("unable to connect to database: %v", err)
	}
	return conn, nil
}

func createTestTable(conn *pgx.Conn) error {
	_, err := conn.Exec(context.Background(), "CREATE TABLE IF NOT EXISTS test_table_base_2 (id SERIAL PRIMARY KEY, data TEXT)")
	if err != nil {
		return fmt.Errorf("unable to create table: %v", err)
	}
	return nil
}

func main() {
	conn, err := connectToDB()
	if err != nil {
		log.Fatalf("Error connecting to database: %v", err)
	}

	defer conn.Close(context.Background())
	if err := createTestTable(conn); err != nil {
		log.Fatalf("Error creating test table: %v", err)
	}

	go httpServer()

	cl, err := kgo.NewClient(
		kgo.SeedBrokers(kafkaAddress),
		kgo.ConsumerGroup("base-2"),
		kgo.ConsumeTopics("received"),
	)

	if err != nil {
		log.Fatalf("Error creating Kafka client: %v", err)
	}
	defer cl.Close()

	ctx := context.Background()

	for {
		log.Println("Polling for messages...")
		fetches := cl.PollFetches(ctx)
		if errs := fetches.Errors(); len(errs) > 0 {
			panic(fmt.Sprint(errs))
		}

		iter := fetches.RecordIter()
		for !iter.Done() {
			record := iter.Next()
			fmt.Println(string(record.Value), "confirmed")
		}
	}
}

func httpServer() {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		log.Printf("Received request: %s %s", r.Method, r.URL.Path)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(exmpl.A))
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
