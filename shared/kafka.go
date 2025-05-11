package shared

const KafkaAddress = "kafka:9092"

const OrderShippingRequestTopic = "order-shipping-request"

type OrderShippingRequestEvent struct {
	OrderID string `json:"order_id"`
}

const OrderShippingStatusTopic = "order-shipping-status"

type OrderShippingStatusEvent struct {
	OrderID string `json:"order_id"`
	Status  string `json:"status"`
}

const OrderRequestTopic = "order-request"

type OrderRequestEvent struct {
	OrderID  string         `json:"order_id"`
	Products map[string]int `json:"products"` // product_id -> amount
}

const OrderRequestResultTopic = "order-request-result"

type OrderRequestResultEvent struct {
	OrderID       string `json:"order_id"`
	RequestStatus string `json:"request_status"`
	Reason        string `json:"reason"`
}

const OrderStateTopic = "order-state"

type OrderStateEvent struct {
	ID       string         `json:"id"`
	Status   string         `json:"status"`
	Products map[string]int `json:"products"`
}

const InventoryStateTopic = "inventory-state"

type InventoryStateEvent struct {
	ID     string `json:"id"`
	Amount int    `json:"amount"`
}
