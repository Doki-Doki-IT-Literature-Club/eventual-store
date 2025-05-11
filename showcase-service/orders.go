package main

import (
	"bytes"
	"html/template"
	"log"
	"net/http"

	"github.com/jackc/pgx/v5"
)

// CONTROLLERS

func handleGetOrdersTableView(conn *pgx.Conn) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log.Printf("Received request: %s %s", r.Method, r.URL.Path)

		orders, err := getOrders(conn, r.Context())
		if err != nil {
			log.Printf("Error fetching orders: %v", err)
			http.Error(w, "Failed to retrieve orders", http.StatusInternalServerError)
			return
		}

		orderHTML := orderTableView(orders)

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(orderHTML))
	}
}

// END CONTROLLERS

// VIEWS

const ordersTemplate = `
{{if .Orders}}
<table border="1" style="border-collapse: collapse; width: 100%;">
  <thead>
    <tr>
      <th>ID</th>
      <th>Status</th>
      <th>Products (ID: Quantity)</th>
    </tr>
  </thead>
  <tbody>
    {{range .Orders}}
    <tr>
      <td>{{.ID}}</td>
      <td>{{.Status}}</td>
      <td>
        {{if .Products}}
          {{range $name, $quantity := .Products}}
            {{$name}}: {{$quantity}}<br>
          {{end}}
        {{else}}
          <em>No products</em>
        {{end}}
      </td>
    </tr>
    {{end}}
  </tbody>
</table>
{{else}}
<p>No orders to display.</p>
{{end}}
`

func orderTableView(orders []Order) template.HTML {
	tmpl, err := template.New("orders").Parse(ordersTemplate)
	if err != nil {
		return template.HTML("<p>Error generating table: " + template.HTMLEscapeString(err.Error()) + "</p>")
	}

	var buf bytes.Buffer
	err = tmpl.Execute(&buf, map[string][]Order{
		"Orders": orders,
	})
	if err != nil {
		return template.HTML("<p>Error generating table: " + template.HTMLEscapeString(err.Error()) + "</p>")
	}

	return template.HTML(buf.String())
}

// END VIEWS
