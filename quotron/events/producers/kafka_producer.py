import json
import logging
import uuid
from typing import Dict, Any, Optional

from kafka import KafkaProducer as KProducer
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

class KafkaProducer:
    """Producer for sending events to Kafka"""
    
    def __init__(self, config: KafkaConfig):
        """Initialize the Kafka producer with the given configuration"""
        self.config = config
        self.producer = KProducer(
            bootstrap_servers=config.bootstrap_servers,
            client_id=config.client_id,
            acks=config.acks,
            retries=config.retries,
            batch_size=config.batch_size,
            linger_ms=config.linger_ms,
            buffer_memory=config.buffer_memory,
            value_serializer=lambda v: json.dumps(v, default=str).encode('utf-8'),
            key_serializer=lambda k: k.encode('utf-8')
        )
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
        
        # Convert event to dict
        event_dict = event.dict()
        
        # Send the message
        future = self.producer.send(
            topic=topic,
            key=key,
            value=event_dict
        )
        
        try:
            # Ensure the message is sent
            record_metadata = future.get(timeout=10)
            logger.debug(f"Message delivered to {record_metadata.topic} [{record_metadata.partition}]")
            logger.info(f"Sent event of type {event.event_type} to topic {topic}")
        except Exception as e:
            logger.error(f"Failed to deliver message: {e}")
    
    def close(self) -> None:
        """Close the producer connection"""
        self.producer.close()
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