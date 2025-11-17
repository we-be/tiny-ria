module github.com/we-be/tiny-ria/agent

go 1.24.1

require (
	github.com/go-redis/redis/v8 v8.11.5
	github.com/gorilla/websocket v1.5.3
	github.com/tiny-ria/quotron/api-service v0.0.0-00010101000000-000000000000
)

require (
	github.com/cespare/xxhash/v2 v2.1.2 // indirect
	github.com/dgryski/go-rendezvous v0.0.0-20200823014737-9f7001d12a5f // indirect
	github.com/lib/pq v1.10.9 // indirect
	github.com/we-be/tiny-ria/quotron/health v0.0.0-00010101000000-000000000000 // indirect
)

replace github.com/tiny-ria/quotron/api-service => ../quotron/api-service

replace github.com/we-be/tiny-ria/quotron/health => ../quotron/health
