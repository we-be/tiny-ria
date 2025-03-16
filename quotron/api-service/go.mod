module github.com/tiny-ria/quotron/api-service

go 1.21

require (
	github.com/gorilla/mux v1.8.1
	github.com/lib/pq v1.10.9
	github.com/rs/cors v1.11.1
	github.com/we-be/tiny-ria/quotron/health v0.0.0-00010101000000-000000000000
)

replace github.com/we-be/tiny-ria/quotron/health => ../health
