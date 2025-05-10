curl http://localhost:8002/orders \
  -X POST \
  -H "Content-Type: application/json" \
  -d '{
    "1": 2
}'