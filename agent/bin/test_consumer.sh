#!/bin/bash

# Simple Redis stream consumer for testing
STREAM_NAME="quotron:alerts:stream"
CONSUMER_GROUP="test-consumer-group"
CONSUMER_ID="test-consumer"

# Create consumer group if it doesn't exist
redis-cli XGROUP CREATE $STREAM_NAME $CONSUMER_GROUP 0 MKSTREAM 2>/dev/null || true

echo "Listening for alerts on stream $STREAM_NAME..."
echo "Press Ctrl+C to stop"

while true; do
  # Read from stream
  RESPONSE=$(redis-cli XREADGROUP GROUP $CONSUMER_GROUP $CONSUMER_ID BLOCK 2000 STREAMS $STREAM_NAME ">")
  
  if [ -n "$RESPONSE" ]; then
    echo "Received alert:"
    echo "$RESPONSE"
    
    # Extract message ID
    MESSAGE_ID=$(echo "$RESPONSE" | awk 'NR==2 {print $1}')
    
    # Acknowledge message
    redis-cli XACK $STREAM_NAME $CONSUMER_GROUP $MESSAGE_ID > /dev/null
    
    echo "Alert processed"
    echo "-----------------------------------"
  fi
  
  sleep 1
done