from datetime import datetime
from typing import Optional, List
from enum import Enum
from pydantic import BaseModel, Field, validator

class DataSource(str, Enum):
    API_SCRAPER = "api-scraper"
    BROWSER_SCRAPER = "browser-scraper"
    MANUAL = "manual"

class Exchange(str, Enum):
    NYSE = "NYSE"
    NASDAQ = "NASDAQ"
    AMEX = "AMEX"
    OTC = "OTC"
    OTHER = "OTHER"

class StockQuote(BaseModel):
    """Schema for stock quote data"""
    symbol: str = Field(..., description="The stock ticker symbol")
    price: float = Field(..., description="Current price of the stock")
    change: float = Field(..., description="Absolute price change")
    change_percent: float = Field(..., description="Percentage price change")
    volume: int = Field(..., description="Trading volume")
    timestamp: datetime = Field(..., description="Time when the quote was recorded")
    exchange: Exchange = Field(..., description="Stock exchange")
    source: DataSource = Field(..., description="Source of the data")
    
    @validator('symbol')
    def symbol_must_be_uppercase(cls, v):
        if not v.isupper():
            return v.upper()
        return v
    
    @validator('price', 'change', 'change_percent')
    def value_must_be_reasonable(cls, v):
        if v > 1_000_000:
            raise ValueError("Value is unreasonably large")
        return v

class MarketIndex(BaseModel):
    """Schema for market index data"""
    name: str = Field(..., description="Name of the market index")
    value: float = Field(..., description="Current value of the index")
    change: float = Field(..., description="Absolute value change")
    change_percent: float = Field(..., description="Percentage value change")
    timestamp: datetime = Field(..., description="Time when the index value was recorded")
    source: DataSource = Field(..., description="Source of the data")

class MarketBatch(BaseModel):
    """Schema for a batch of market data"""
    quotes: List[StockQuote] = Field(default_factory=list, description="List of stock quotes")
    indices: List[MarketIndex] = Field(default_factory=list, description="List of market indices")
    batch_id: str = Field(..., description="Unique identifier for the batch")
    created_at: datetime = Field(default_factory=datetime.utcnow, description="Batch creation timestamp")
    
    @validator('batch_id')
    def batch_id_must_be_valid(cls, v):
        if not v or len(v) < 8:
            raise ValueError("Batch ID must be at least 8 characters")
        return v