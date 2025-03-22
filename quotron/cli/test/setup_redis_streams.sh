#!/bin/bash

# This script initializes Redis streams and consumer groups for Quotron ETL
# It should be run before starting the ETL service to ensure all streams exist

set -e

REDIS_HOST="localhost"
REDIS_PORT="6379"
REDIS_CLI="redis-cli -h $REDIS_HOST -p $REDIS_PORT"

# Redis streams and consumer group
STOCK_STREAM="quotron:stocks:stream"
CRYPTO_STREAM="quotron:crypto:stream"
INDEX_STREAM="quotron:indices:stream"
CONSUMER_GROUP="quotron:etl"

# Colors for prettier output
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
RED='\033[0;31m'
NC='\033[0m' # No Color

echo -e "${GREEN}Setting up Redis streams and consumer groups for ETL...${NC}"

# Function to initialize a stream and consumer group
# Takes a stream name as parameter
initialize_stream() {
    local stream=$1
    echo -e "\n${YELLOW}Initializing stream: $stream${NC}"
    
    # Check if stream exists
    if $REDIS_CLI exists $stream > /dev/null; then
        echo "Stream $stream already exists"
    else
        # Create stream with a dummy message
        echo "Creating stream $stream..."
        $REDIS_CLI XADD $stream MAXLEN 1000 * init true
        echo -e "${GREEN}Stream $stream created successfully${NC}"
    fi
    
    # Get stream information
    echo "Stream info:"
    $REDIS_CLI XINFO STREAM $stream || true
    
    # Create consumer group if it doesn't exist
    echo "Creating consumer group $CONSUMER_GROUP for stream $stream..."
    $REDIS_CLI XGROUP CREATE $stream $CONSUMER_GROUP 0 MKSTREAM 2>/dev/null || echo "Consumer group already exists"
    
    # Get consumer group information
    echo "Consumer group info:"
    $REDIS_CLI XINFO GROUPS $stream || true
}

# Initialize all streams
initialize_stream $STOCK_STREAM
initialize_stream $CRYPTO_STREAM
initialize_stream $INDEX_STREAM

# Verify subscriptions (list all subscribers to these channels)
echo -e "\n${YELLOW}Verifying Redis PubSub subscriptions:${NC}"
$REDIS_CLI PUBSUB CHANNELS "quotron:*"
$REDIS_CLI PUBSUB NUMSUB "quotron:stocks" "quotron:crypto" "quotron:indices"

echo -e "\n${GREEN}Redis setup complete!${NC}"
echo "You can now start the ETL service and it should be able to process messages from all streams."
echo "To monitor Redis activity, run: ./crypto_redis_monitor"