module github.com/Doki-Doki-IT-Literature-Club/sops/shipment-service

go 1.23.4

require github.com/jackc/pgx/v5 v5.7.4 // indirect

require github.com/google/uuid v1.6.0 // indirect

require (
	github.com/Doki-Doki-IT-Literature-Club/sops/shared v1.0.0
	github.com/jackc/pgpassfile v1.0.0 // indirect
	github.com/jackc/pgservicefile v0.0.0-20240606120523-5a60cdf6a761 // indirect
	github.com/klauspost/compress v1.17.11 // indirect
	github.com/pierrec/lz4/v4 v4.1.22 // indirect
	github.com/twmb/franz-go v1.18.1 // indirect
	github.com/twmb/franz-go/pkg/kmsg v1.9.0 // indirect
	golang.org/x/crypto v0.32.0 // indirect
	golang.org/x/text v0.21.0 // indirect
)

replace github.com/Doki-Doki-IT-Literature-Club/sops/shared => ../shared
