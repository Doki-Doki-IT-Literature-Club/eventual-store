package main

import (
	"base/pkg/exmpl"
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/twmb/franz-go/pkg/kgo"
)

var base2Address = "http://base-2-base-2-1:8002"
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
	_, err := conn.Exec(context.Background(), "CREATE TABLE IF NOT EXISTS test_table_base_1 (id SERIAL PRIMARY KEY, data TEXT)")
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

	kcl, err := kgo.NewClient(
		kgo.SeedBrokers(kafkaAddress),
	)

	if err != nil {
		log.Fatalf("Error creating Kafka client: %v", err)
	}

	httpServer(kcl)
}

func httpServer(kcl *kgo.Client) {
	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte("404 Not Found"))
			return
		}

		log.Printf("Received request: %s %s", r.Method, r.URL.Path)

		req, err := http.NewRequest(http.MethodPost, base2Address, bytes.NewBufferString(exmpl.A))
		if err != nil {
			log.Printf("Error creating request: %v", err)
			http.Error(w, "Error creating request", http.StatusInternalServerError)
			return
		}
		req.Header.Set("Content-Type", "text/plain")

		resp, err := client.Do(req)
		if err != nil {
			log.Printf("Error sending request to base-2: %v", err)
			http.Error(w, "Error sending request to base-2", http.StatusInternalServerError)
			return
		}
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			log.Printf("Error reading response: %v", err)
			http.Error(w, "Error reading response", http.StatusInternalServerError)
			return
		}

		ctx := context.Background()
		record := &kgo.Record{Topic: "received", Value: body}
		kcl.Produce(ctx, record, func(_ *kgo.Record, err error) {
			if err != nil {
				fmt.Printf("record had a produce error: %v\n", err)
			}
		})

		w.WriteHeader(resp.StatusCode)
		w.Write(body)
		log.Printf("Response from base-2: %s", string(body))
	})

	port := 8001
	log.Printf("Starting server on port %d", port)
	log.Printf("Ready to send %q to %q", exmpl.A, base2Address)
	if err := http.ListenAndServe(fmt.Sprintf(":%d", port), nil); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}
