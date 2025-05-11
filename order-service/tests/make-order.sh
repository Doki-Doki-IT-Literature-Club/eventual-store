curl http://localhost:8002/api/orders \
  -X POST \
  -H "Content-Type: application/json" \
  -d '{
    "products": {
      "product1": 1
    }
}'