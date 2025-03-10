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
    change_percent: Optional[float] = Field(0.0, description="Percentage price change")
    changePercent: Optional[float] = Field(0.0, description="Percentage price change (alternative field name)")
    volume: int = Field(..., description="Trading volume")
    timestamp: datetime = Field(..., description="Time when the quote was recorded")
    exchange: str = Field(..., description="Stock exchange")
    source: str = Field(..., description="Source of the data")
    
    @validator('source', pre=True)
    def validate_source(cls, v):
        # Try to convert string source to DataSource enum
        try:
            return DataSource(v)
        except ValueError:
            # If it's not one of our predefined sources, return as is
            return v
            
    @validator('exchange', pre=True)
    def validate_exchange(cls, v):
        # Try to convert string exchange to Exchange enum
        try:
            return Exchange(v)
        except ValueError:
            # If it's not one of our predefined exchanges, return as is
            return v
    
    class Config:
        # Allow extra fields
        extra = "ignore"
        
    @validator('change_percent', 'changePercent', pre=True, always=True)
    def merge_percent_fields(cls, v, values):
        # Get the value from one of the percent fields
        if v is not None:
            return v
        elif 'changePercent' in values and values['changePercent'] is not None:
            return values['changePercent']
        elif 'change_percent' in values and values['change_percent'] is not None:
            return values['change_percent']
        return v
    
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
    name: str = Field(None, description="Name of the market index")
    indexName: str = Field(None, description="Name of the market index (alternative field)")
    value: float = Field(..., description="Current value of the index")
    change: float = Field(..., description="Absolute value change")
    change_percent: Optional[float] = Field(0.0, description="Percentage value change")
    changePercent: Optional[float] = Field(0.0, description="Percentage value change (alternative field)")
    timestamp: datetime = Field(..., description="Time when the index value was recorded")
    source: str = Field(..., description="Source of the data")
    
    class Config:
        # Allow extra fields
        extra = "ignore"
    
    @validator('name', 'indexName', pre=True, always=True)
    def merge_name_fields(cls, v, values):
        if v is not None:
            return v
        elif 'indexName' in values and values['indexName'] is not None:
            return values['indexName']
        elif 'name' in values and values['name'] is not None:
            return values['name']
        return v
        
    @validator('change_percent', 'changePercent', pre=True, always=True)
    def merge_percent_fields(cls, v, values):
        if v is not None:
            return v
        elif 'changePercent' in values and values['changePercent'] is not None:
            return values['changePercent']
        elif 'change_percent' in values and values['change_percent'] is not None:
            return values['change_percent']
        return v
        
    @validator('source', pre=True)
    def validate_source(cls, v):
        try:
            return DataSource(v)
        except ValueError:
            return v

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