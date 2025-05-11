module github.com/Doki-Doki-IT-Literature-Club/sops/order-service

go 1.24.3

require (
	github.com/Doki-Doki-IT-Literature-Club/sops/shared v1.0.0
	github.com/google/uuid v1.6.0
	github.com/twmb/franz-go v1.19.1
)

replace github.com/Doki-Doki-IT-Literature-Club/sops/shared => ../shared

require (
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/jackc/pgpassfile v1.0.0 // indirect
	github.com/jackc/pgservicefile v0.0.0-20240606120523-5a60cdf6a761 // indirect
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
	github.com/prometheus/client_model v0.6.1 // indirect
	github.com/prometheus/common v0.62.0 // indirect
	github.com/prometheus/procfs v0.15.1 // indirect
	golang.org/x/crypto v0.38.0 // indirect
	golang.org/x/sys v0.33.0 // indirect
	golang.org/x/text v0.25.0 // indirect
	google.golang.org/protobuf v1.36.5 // indirect
)

require (
	github.com/jackc/pgx/v5 v5.7.4
	github.com/klauspost/compress v1.18.0 // indirect
	github.com/pierrec/lz4/v4 v4.1.22 // indirect
	github.com/prometheus/client_golang v1.22.0
	github.com/twmb/franz-go/pkg/kmsg v1.11.2 // indirect
)
