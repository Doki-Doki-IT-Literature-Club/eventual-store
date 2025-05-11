package main

import (
	"context"
	"log"

	"github.com/Doki-Doki-IT-Literature-Club/sops/shared"
)

func main() {
	log.Printf("Starting showcase service")

	ctx := context.Background()
	kcl, err := initKafkaClient()
	if err != nil {
		log.Fatalf("Error creating Kafka client: %v", err)
	}
	defer kcl.Close()

	if err != nil {
		log.Fatalf("Error creating Kafka client: %v", err)
	}

	err = shared.EnsureDBExists(ctx, dbName)
	if err != nil {
		log.Fatalf("Error ensuring database exists: %v", err)
	}
	conn, err := shared.ConnectToDB(ctx, dbName)
	if err != nil {
		log.Fatalf("Error connecting to database: %v", err)
	}

	defer conn.Close(ctx)

	if err := initDB(ctx, conn); err != nil {
		log.Fatalf("Error creating test table: %v", err)
	}

	go consume(kcl, conn, ctx)
	httpServer(conn)
}
