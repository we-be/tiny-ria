module github.com/tiny-ria/quotron/api-service

go 1.21

require (
	github.com/google/uuid v1.6.0
	github.com/gorilla/mux v1.8.1
	github.com/lib/pq v1.10.9
	github.com/pkg/errors v0.9.1
	github.com/rs/cors v1.11.1
	github.com/tiny-ria/quotron/api-scraper v0.0.0-00010101000000-000000000000
	github.com/we-be/tiny-ria/quotron/health v0.0.0-00010101000000-000000000000
)

require (
	github.com/piquette/finance-go v1.1.0 // indirect
	github.com/shopspring/decimal v0.0.0-20180709203117-cd690d0c9e24 // indirect
	github.com/stretchr/testify v1.9.0 // indirect
	github.com/we-be/tiny-ria/quotron/api-scraper v0.0.0-20250405192517-2f7466dbc1cb // indirect
)

replace github.com/we-be/tiny-ria/quotron/health => ../health

// Make the server package available
replace github.com/tiny-ria/quotron/api-service/cmd/server => ./cmd/server

replace github.com/tiny-ria/quotron/api-scraper => ../api-scraper
