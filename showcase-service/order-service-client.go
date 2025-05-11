package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/Doki-Doki-IT-Literature-Club/sops/shared"
)

const address = "http://order-service:80"

type OrderServiceClient struct{}

func NewOrderServiceClient() *OrderServiceClient {
	return &OrderServiceClient{}
}

func (c *OrderServiceClient) Order(payload shared.CreateOrderPayload) (string, error) {

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("failed to marshal payload: %w", err)
	}

	resp, err := http.Post(
		fmt.Sprintf("%s/api/orders", address),
		"application/json",
		bytes.NewBuffer(jsonData),
	)
	if err != nil {
		return "", fmt.Errorf("failed to send order request: %w", err)
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to place order, status code: %d", resp.StatusCode)
	}

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %w", err)
	}

	return string(responseBody), nil
}
