package main

import (
	"bytes"
	"html/template"
	"log"
	"net/http"

	"github.com/jackc/pgx/v5"
)

// // CONTROLLERS

func handleGetInventoryTableView(conn *pgx.Conn) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log.Printf("Received request: %s %s", r.Method, r.URL.Path)

		inventory, err := getInventory(conn, r.Context())
		if err != nil {
			log.Printf("Error fetching inventory: %v", err)
			http.Error(w, "Failed to retrieve inventory", http.StatusInternalServerError)
			return
		}

		inventoryHTML := inventoryTableView(inventory)

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(inventoryHTML))
	}
}

// END CONTROLLERS

// VIEWS

const inventoryTemplate = `
{{if .Inventory}}
<table border="1" style="border-collapse: collapse; width: 100%;">
  <thead>
	<tr>
	  <th>ID</th>
	  <th>Amount</th>
	</tr>
  </thead>
  <tbody>
	{{range .Inventory}}
	<tr>
	  <td>{{.ID}}</td>
	  <td>{{.Amount}}</td>
	</tr>
	{{end}}
  </tbody>
</table>
{{else}}
<p>No inventory data available.</p>
{{end}}
`

func inventoryTableView(inventory []Inventory) template.HTML {
	tmpl, err := template.New("inventory").Parse(inventoryTemplate)
	if err != nil {
		return template.HTML("<p>Error generating table: " + template.HTMLEscapeString(err.Error()) + "</p>")
	}

	var buf bytes.Buffer
	err = tmpl.Execute(&buf, map[string][]Inventory{
		"Inventory": inventory,
	})
	if err != nil {
		return template.HTML("<p>Error generating table: " + template.HTMLEscapeString(err.Error()) + "</p>")
	}

	return template.HTML(buf.String())
}

// END VIEWS
