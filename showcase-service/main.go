package main

import (
	"context"
	"encoding/json"
	"html"
	"log"
	"strings"

	"fmt"
	"net/http"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/twmb/franz-go/pkg/kgo"
)

const (
	kafkaAddress    = "kafka:9092"
	dbConnString    = "postgres://user:password@postgres:5432/"
	dbName          = "showcase_db"
	orderStateTopic = "order-state"
)

type Order struct {
	ID       string         `json:"id"`
	Status   string         `json:"status"`
	Products map[string]int `json:"products"`
}

func OrdersToHTMLTable(orders []Order) string {
	if len(orders) == 0 {
		return "<p>No orders to display.</p>" // Or return an empty table structure
	}

	var sb strings.Builder

	sb.WriteString("<table border=\"1\" style=\"border-collapse: collapse; width: 100%;\">\n")

	sb.WriteString("  <thead>\n")
	sb.WriteString("    <tr>\n")
	sb.WriteString("      <th>ID</th>\n")
	sb.WriteString("      <th>Status</th>\n")
	sb.WriteString("      <th>Products (ID: Quantity)</th>\n")
	sb.WriteString("    </tr>\n")
	sb.WriteString("  </thead>\n")

	sb.WriteString("  <tbody>\n")
	for _, order := range orders {
		sb.WriteString("    <tr>\n")

		sb.WriteString("      <td>")
		sb.WriteString(html.EscapeString(order.ID))
		sb.WriteString("</td>\n")

		sb.WriteString("      <td>")
		sb.WriteString(html.EscapeString(order.Status))
		sb.WriteString("</td>\n")

		sb.WriteString("      <td>")
		if len(order.Products) > 0 {
			productNames := make([]string, 0, len(order.Products))
			for name := range order.Products {
				productNames = append(productNames, name)
			}

			productEntries := make([]string, 0, len(productNames))
			for _, name := range productNames {
				escapedName := html.EscapeString(name)
				quantity := order.Products[name]
				productEntries = append(productEntries, fmt.Sprintf("%s: %d", escapedName, quantity))
			}
			sb.WriteString(strings.Join(productEntries, "<br>\n"))
		} else {
			sb.WriteString("<em>No products</em>")
		}
		sb.WriteString("</td>\n")

		sb.WriteString("    </tr>\n")
	}
	sb.WriteString("  </tbody>\n")
	sb.WriteString("</table>\n")

	return sb.String()
}

func httpServer(conn *pgx.Conn, kcl *kgo.Client, ctx context.Context) {
	http.HandleFunc("GET /orders", func(w http.ResponseWriter, r *http.Request) {
		log.Printf("Received request: %s %s", r.Method, r.URL.Path)
		orders, err := GetOrders(conn, context.Background())
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
		}
		w.WriteHeader(http.StatusOK)
		orderSting, err := json.Marshal(orders)
		w.Write([]byte(orderSting))
	})
	http.HandleFunc("GET /orders-html", func(w http.ResponseWriter, r *http.Request) {
		log.Printf("Received request: %s %s", r.Method, r.URL.Path)
		orders, err := GetOrders(conn, context.Background())
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
		}
		w.WriteHeader(http.StatusOK)
		orderSting := OrdersToHTMLTable(orders)
		w.Write([]byte(orderSting))
	})

	port := 80
	log.Printf("Starting server on port %d", port)
	if err := http.ListenAndServe(fmt.Sprintf(":%d", port), nil); err != nil {
		log.Fatalf("Server error: %v", err)

		if err != nil {
			panic(err)
		}
	}
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
			log.Printf("New record")
			if r.Topic == orderStateTopic {
				log.Printf("Processing record from topic %s", r.Topic)

				order := &Order{}
				if err := json.Unmarshal(r.Value, order); err != nil {
					log.Printf("Error unmarshalling record: %v", err)
					return
				}

				err := updateOrder(conn, ctx, *order)
				if err != nil {
					log.Printf(err.Error())
				}
			} else {
				log.Printf("Record from topic %s, skipping for now...", r.Topic)
			}
		})
	}
}

func connectToDB() (*pgx.Conn, error) {
	conn, err := pgx.Connect(context.Background(), dbConnString+"/"+dbName)
	if err != nil {
		return nil, fmt.Errorf("unable to connect to database: %v", err)
	}
	return conn, nil
}

func initDB(conn *pgx.Conn) error {
	tableSQL := `
		CREATE TABLE IF NOT EXISTS orders
	  (id TEXT PRIMARY KEY, status TEXT, products TEXT)`
	_, err := conn.Exec(context.Background(), tableSQL)
	if err != nil {
		return fmt.Errorf("unable to create table: %v", err)
	}
	return nil
}

func updateOrder(conn *pgx.Conn, ctx context.Context, order Order) error {
	log.Print("Starting updating")
	query := `
	INSERT INTO orders (id, status, products) 
	VALUES ($1, $2, $3) 
	ON CONFLICT (id)
	DO UPDATE SET status = $2, products = $3;
	`
	stringProducs, err := json.Marshal(order.Products)
	_, err = conn.Exec(context.Background(), query, order.ID, order.Status, stringProducs)
	if err != nil {
		return fmt.Errorf("unable to update: %v", err)
	}
	return nil
}

func GetOrders(conn *pgx.Conn, ctx context.Context) ([]Order, error) {
	log.Print("Starting select")
	selectSQL := `
	SELECT id, products, status
	FROM orders 
	`
	rows, err := conn.Query(ctx, selectSQL)
	if err != nil {
		return nil, fmt.Errorf("error querying orders: %w", err)
	}
	defer rows.Close()

	orders := make([]Order, 0, 0)
	for rows.Next() {
		var order Order
		var products string
		var productsMap map[string]int
		if err := rows.Scan(&order.ID, &products, &order.Status); err != nil {
			return nil, fmt.Errorf("error scanning row: %w", err)
		}
		err := json.Unmarshal([]byte(products), &productsMap)
		if err != nil {
			return nil, fmt.Errorf("error formatting products: %w", err)
		}
		order.Products = productsMap
		orders = append(orders, order)
	}
	return orders, nil
}

func main() {
	log.Printf("Starting order service")

	ctx := context.Background()
	kcl, err := kgo.NewClient(
		kgo.SeedBrokers(kafkaAddress),
		kgo.AllowAutoTopicCreation(),
		kgo.ConsumerGroup("order-group"),
		kgo.ConsumeTopics(
			orderStateTopic,
		),
	)

	if err != nil {
		log.Fatalf("Error creating Kafka client: %v", err)
	}

	ensureDBExists(dbConnString, dbName)

	conn, err := connectToDB()
	if err != nil {
		log.Fatalf("Error connecting to database: %v", err)
	}

	defer conn.Close(ctx)

	if err := initDB(conn); err != nil {
		log.Fatalf("Error creating test table: %v", err)
	}

	defer kcl.Close()
	go consume(kcl, conn, ctx)
	httpServer(conn, kcl, ctx)
}

func ensureDBExists(dbConnString string, dbName string) {
	initialDbConnString := dbConnString + "postgres"
	conn, err := pgx.Connect(context.Background(), initialDbConnString)
	if err != nil {
		log.Fatalf("Unable to connect to database: %v", err)
	}
	defer conn.Close(context.Background())

	_, err = conn.Exec(context.Background(), fmt.Sprintf("CREATE DATABASE %s", pgx.Identifier{dbName}.Sanitize()))
	if err != nil {
		pgErr, ok := err.(*pgconn.PgError)
		// 42P04 = duplicate_database'
		if ok && pgErr.Code == "42P04" {
			log.Printf("Database '%s' exists.\n", dbName)
		} else {
			log.Fatalf("Failed to create database '%s': %v\n", dbName, err)
		}
	} else {
		log.Printf("Database '%s' created.\n", dbName)
	}

	log.Printf("Database '%s' checked.\n", dbName)
}
