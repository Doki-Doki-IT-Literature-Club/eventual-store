package main

import (
	"bytes"
	"encoding/json"
	"html/template"
	"log"
	"net/http"

	"github.com/Doki-Doki-IT-Literature-Club/sops/shared"

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

type OrderRequestBody struct {
	ID     string `json:"id"`
	Amount int    `json:"amount"`
}

func handlePostInventoryOrder() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log.Printf("Received request: %s %s", r.Method, r.URL.Path)

		var requestBody OrderRequestBody
		if err := json.NewDecoder(r.Body).Decode(&requestBody); err != nil {
			log.Printf("Error decoding request body: %v", err)
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}
		if requestBody.Amount <= 0 {
			log.Printf("Invalid amount: %d", requestBody.Amount)
			http.Error(w, "Invalid amount", http.StatusBadRequest)
			return
		}

		orderServiceClient := NewOrderServiceClient()
		payload := shared.CreateOrderPayload{}
		payload.Products = map[string]int{requestBody.ID: requestBody.Amount}

		response, err := orderServiceClient.Order(payload)
		if err != nil {
			log.Printf("Error placing order: %v", err)
			http.Error(w, "Failed to place order", http.StatusInternalServerError)
			return
		}
		log.Printf("Order response: %s", response)

		w.WriteHeader(http.StatusOK)
		w.Write([]byte(response))
	}
}

// END CONTROLLERS

// VIEWS

const inventoryTemplate = `
{{if .Inventory}}
<script>

function onSubmit(event) {
  event.preventDefault();
  const form = event.target;
  const id = form.querySelector('input[name="id"]').value;
  const amount = Number(form.querySelector('input[name="amount"]').value);
  const url = '/api/inventory/order';
  const data = { id: id, amount: amount };
  fetch(url, {
    method: 'POST',
	headers: {
	  'Content-Type': 'application/json'
	},
	body: JSON.stringify(data)
  }).then(response => {
    if (response.ok) {
	  alert('Order placed successfully');
	} else {
	  alert('Failed to place order');
	}
  }).catch(error => {
	console.error('Error:', error);
	alert('Error placing order');
  })
};



function orderInventory(id) {
  fetch('/api/inventory/' + id + '/order', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json'
    }
  })
  .then(response => {
    if (response.ok) {
      alert('Order placed successfully');
      // Refresh the page to update inventory
      location.reload();
    } else {
      alert('Failed to place order');
    }
  })
  .catch(error => {
    console.error('Error:', error);
    alert('Error placing order');
  });
}
</script>
<table border="1" style="border-collapse: collapse; width: 100%;">
  <thead>
	<tr>
	  <th>ID</th>
	  <th>Amount</th>
		<th>Actions</th>
	</tr>
  </thead>
  <tbody>
	{{range .Inventory}}
	<tr>
	  <td>{{.ID}}</td>
	  <td>{{.Amount}}</td>
	  <td>
		<form method="POST" onsubmit="onSubmit(event)">
		  <input type="hidden" name="id" value="{{.ID}}" />
		  <input type="number" name="amount" value="1" min="1" />
		  <button type="submit">Order</button>
		</form>
	  </td>
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
