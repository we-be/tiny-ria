import json
import logging
import uuid
from typing import Dict, Any, Optional

from confluent_kafka import Producer
from pydantic import BaseModel

from ..schemas.event_schema import Event

# Configure logging
logging.basicConfig(
    level=logging.INFO,
    format="%(asctime)s - %(name)s - %(levelname)s - %(message)s",
)
logger = logging.getLogger(__name__)

class KafkaConfig(BaseModel):
    """Configuration for Kafka connection"""
    bootstrap_servers: str
    client_id: str
    acks: str = "all"
    retries: int = 3
    batch_size: int = 16384
    linger_ms: int = 5
    buffer_memory: int = 33554432
    key_serializer: str = "string"
    value_serializer: str = "json"

class KafkaProducer:
    """Producer for sending events to Kafka"""
    
    def __init__(self, config: KafkaConfig):
        """Initialize the Kafka producer with the given configuration"""
        self.config = config
        self.producer = Producer({
            'bootstrap.servers': config.bootstrap_servers,
            'client.id': config.client_id,
            'acks': config.acks,
            'retries': config.retries,
            'batch.size': config.batch_size,
            'linger.ms': config.linger_ms,
            'buffer.memory': config.buffer_memory,
        })
        logger.info(f"Initialized Kafka producer with bootstrap servers: {config.bootstrap_servers}")
    
    def send_event(self, topic: str, event: Event, key: Optional[str] = None) -> None:
        """
        Send an event to a Kafka topic
        
        Args:
            topic: Kafka topic to send the event to
            event: Event object to send
            key: Optional key for the message
        """
        if key is None:
            key = str(uuid.uuid4())
        
        # Convert event to JSON string
        event_json = json.dumps(event.dict(), default=str)
        
        # Define delivery callback
        def delivery_callback(err, msg):
            if err:
                logger.error(f"Failed to deliver message: {err}")
            else:
                logger.debug(f"Message delivered to {msg.topic()} [{msg.partition()}]")
        
        # Produce the message
        self.producer.produce(
            topic=topic,
            key=key,
            value=event_json,
            callback=delivery_callback
        )
        
        # Flush to ensure the message is sent
        self.producer.flush(timeout=10)
        
        logger.info(f"Sent event of type {event.event_type} to topic {topic}")
    
    def close(self) -> None:
        """Close the producer connection"""
        self.producer.flush()
        logger.info("Kafka producer closed")

# Example usage
if __name__ == "__main__":
    from ..schemas.event_schema import StockQuoteEvent
    
    # Configuration
    config = KafkaConfig(
        bootstrap_servers="localhost:9092",
        client_id="quotron-producer"
    )
    
    # Create producer
    producer = KafkaProducer(config)
    
    # Create and send an event
    event = StockQuoteEvent(
        event_id=str(uuid.uuid4()),
        source="quotron-example",
        data={
            "symbol": "AAPL",
            "price": 150.25,
            "change": 2.5,
            "change_percent": 1.69,
            "volume": 12345678,
        },
        metadata={
            "environment": "development",
        }
    )
    
    producer.send_event("stock-quotes", event)
    producer.close()