#!/bin/bash
cd "$(dirname "$0")"
nohup /home/hunter/Desktop/tiny-ria/quotron/cli/cmd/etl/etl -start -redis=localhost:6379 -dbhost=localhost -dbport=5432 -dbname=quotron -dbuser=quotron -dbpass=quotron -workers=2 >> /tmp/etl_service.log 2>&1 &
echo $! > /home/hunter/Desktop/tiny-ria/quotron/cli/.etl_service.pid
