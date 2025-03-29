module github.com/we-be/tiny-ria/agent

go 1.24.1

require (
	github.com/go-redis/redis/v8 v8.11.5
	github.com/gorilla/websocket v1.5.3
	github.com/we-be/tiny-ria/quotron/scheduler v0.0.0
)

require (
	github.com/cespare/xxhash/v2 v2.1.2 // indirect
	github.com/dgryski/go-rendezvous v0.0.0-20200823014737-9f7001d12a5f // indirect
)

replace github.com/we-be/tiny-ria/quotron/scheduler => ../quotron/scheduler
