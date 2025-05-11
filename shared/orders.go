package shared

type CreateOrderPayload struct {
	Products map[string]int `json:"products"`
}
