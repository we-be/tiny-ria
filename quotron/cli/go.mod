module github.com/we-be/tiny-ria/quotron/cli

go 1.24.1

require github.com/we-be/tiny-ria/quotron/health v0.0.0

require (
	github.com/lib/pq v1.10.9
	github.com/redis/go-redis/v9 v9.7.1
	github.com/robfig/cron/v3 v3.0.1
)

require (
	github.com/cespare/xxhash/v2 v2.2.0 // indirect
	github.com/dgryski/go-rendezvous v0.0.0-20200823014737-9f7001d12a5f // indirect
)

replace github.com/we-be/tiny-ria/quotron/health => ../health
