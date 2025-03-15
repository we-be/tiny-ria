module github.com/we-be/tiny-ria/quotron/cli

go 1.24.1

require github.com/we-be/tiny-ria/quotron/health v0.0.0

require github.com/lib/pq v1.10.9 // indirect

replace github.com/we-be/tiny-ria/quotron/health => ../health
