from datetime import datetime
from enum import Enum
from typing import Dict, Any, Optional, List
from pydantic import BaseModel, Field

class EventType(str, Enum):
    STOCK_QUOTE_UPDATED = "stock_quote_updated"
    MARKET_INDEX_UPDATED = "market_index_updated"
    DATA_BATCH_PROCESSED = "data_batch_processed"
    SCRAPING_SCHEDULED = "scraping_scheduled"
    SCRAPING_COMPLETED = "scraping_completed"
    SCRAPING_FAILED = "scraping_failed"
    SYSTEM_ERROR = "system_error"

class EventPriority(str, Enum):
    LOW = "low"
    NORMAL = "normal"
    HIGH = "high"
    CRITICAL = "critical"

class Event(BaseModel):
    """Base schema for all events"""
    event_id: str = Field(..., description="Unique event identifier")
    event_type: EventType = Field(..., description="Type of the event")
    timestamp: datetime = Field(default_factory=datetime.utcnow, description="When the event occurred")
    priority: EventPriority = Field(default=EventPriority.NORMAL, description="Event priority")
    source: str = Field(..., description="Component that generated the event")
    data: Dict[str, Any] = Field(..., description="Event payload")
    metadata: Dict[str, Any] = Field(default_factory=dict, description="Additional metadata")

class StockQuoteEvent(Event):
    """Event for stock quote updates"""
    def __init__(self, **data):
        super().__init__(
            event_type=EventType.STOCK_QUOTE_UPDATED,
            **data
        )

class MarketIndexEvent(Event):
    """Event for market index updates"""
    def __init__(self, **data):
        super().__init__(
            event_type=EventType.MARKET_INDEX_UPDATED,
            **data
        )

class BatchProcessedEvent(Event):
    """Event for when a batch of data is processed"""
    def __init__(self, **data):
        super().__init__(
            event_type=EventType.DATA_BATCH_PROCESSED,
            **data
        )

class ScrapingScheduledEvent(Event):
    """Event for when scraping is scheduled"""
    def __init__(self, **data):
        super().__init__(
            event_type=EventType.SCRAPING_SCHEDULED,
            **data
        )

class ScrapingCompletedEvent(Event):
    """Event for when scraping is completed"""
    def __init__(self, **data):
        super().__init__(
            event_type=EventType.SCRAPING_COMPLETED,
            **data
        )

class ScrapingFailedEvent(Event):
    """Event for when scraping fails"""
    def __init__(self, **data):
        super().__init__(
            event_type=EventType.SCRAPING_FAILED,
            **data
        )

class SystemErrorEvent(Event):
    """Event for system errors"""
    def __init__(self, **data):
        super().__init__(
            event_type=EventType.SYSTEM_ERROR,
            priority=EventPriority.CRITICAL,
            **data
        )