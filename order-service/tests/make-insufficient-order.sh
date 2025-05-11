curl http://localhost:8002/api/orders \
  -X POST \
  -H "Content-Type: application/json" \
  -d '{
    "products": {
      "product3": 5
    }
}'