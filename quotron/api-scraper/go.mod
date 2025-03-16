module github.com/we-be/tiny-ria/quotron/api-scraper

go 1.21

require (
	github.com/pkg/errors v0.9.1
	github.com/we-be/tiny-ria/quotron/health v0.0.0-00010101000000-000000000000
)

require (
	github.com/gorilla/mux v1.8.1 // indirect
	github.com/lib/pq v1.10.9 // indirect
	github.com/piquette/finance-go v1.1.0 // indirect
	github.com/shopspring/decimal v0.0.0-20180709203117-cd690d0c9e24 // indirect
)

replace github.com/we-be/tiny-ria/quotron/health => ../health
