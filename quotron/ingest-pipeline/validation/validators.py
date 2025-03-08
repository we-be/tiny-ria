import logging
import pandas as pd
from datetime import datetime, timedelta
from typing import Dict, List, Any, Optional

from ..schemas.finance_schema import StockQuote, MarketIndex, MarketBatch

# Configure logging
logging.basicConfig(
    level=logging.INFO,
    format="%(asctime)s - %(name)s - %(levelname)s - %(message)s",
)
logger = logging.getLogger(__name__)

class DataValidator:
    """Validator for financial data"""
    
    def __init__(self):
        """Initialize the validator with reasonable bounds for financial data"""
        self.price_max = 100000.0  # Max reasonable stock price
        self.price_min = 0.0001    # Min reasonable stock price
        self.volume_max = 10000000000  # Max reasonable volume (10B)
        self.yesterday = datetime.utcnow() - timedelta(days=1)
    
    def validate_stock_quote(self, data: Dict[str, Any]) -> Optional[StockQuote]:
        """
        Validates a stock quote and returns a StockQuote object if valid.
        Returns None if validation fails.
        """
        try:
            # Perform basic validation with Pydantic schema
            quote = StockQuote(**data)
            
            # Additional validation
            if not self._is_price_reasonable(quote.price):
                logger.warning(f"Unreasonable price for {quote.symbol}: {quote.price}")
                return None
            
            if not self._is_volume_reasonable(quote.volume):
                logger.warning(f"Unreasonable volume for {quote.symbol}: {quote.volume}")
                return None
            
            if not self._is_timestamp_recent(quote.timestamp):
                logger.warning(f"Outdated timestamp for {quote.symbol}: {quote.timestamp}")
                return None
            
            logger.info(f"Quote for {quote.symbol} passed validation")
            return quote
        
        except Exception as e:
            logger.error(f"Validation failed for stock quote: {e}")
            return None
    
    def validate_market_index(self, data: Dict[str, Any]) -> Optional[MarketIndex]:
        """
        Validates a market index and returns a MarketIndex object if valid.
        Returns None if validation fails.
        """
        try:
            # Perform basic validation with Pydantic schema
            index = MarketIndex(**data)
            
            # Additional validation
            if not self._is_timestamp_recent(index.timestamp):
                logger.warning(f"Outdated timestamp for {index.name}: {index.timestamp}")
                return None
            
            logger.info(f"Index {index.name} passed validation")
            return index
        
        except Exception as e:
            logger.error(f"Validation failed for market index: {e}")
            return None
    
    def validate_batch(self, quotes: List[Dict[str, Any]], indices: List[Dict[str, Any]]) -> MarketBatch:
        """
        Validates a batch of market data and returns a MarketBatch object.
        Filters out invalid entries.
        """
        valid_quotes = []
        for quote_data in quotes:
            quote = self.validate_stock_quote(quote_data)
            if quote:
                valid_quotes.append(quote)
        
        valid_indices = []
        for index_data in indices:
            index = self.validate_market_index(index_data)
            if index:
                valid_indices.append(index)
        
        batch_id = f"batch_{datetime.utcnow().strftime('%Y%m%d%H%M%S')}"
        batch = MarketBatch(quotes=valid_quotes, indices=valid_indices, batch_id=batch_id)
        
        logger.info(f"Created batch {batch_id} with {len(valid_quotes)} quotes and {len(valid_indices)} indices")
        return batch
    
    def _is_price_reasonable(self, price: float) -> bool:
        """Check if a price is within reasonable bounds"""
        return self.price_min <= price <= self.price_max
    
    def _is_volume_reasonable(self, volume: int) -> bool:
        """Check if a volume is within reasonable bounds"""
        return 0 <= volume <= self.volume_max
    
    def _is_timestamp_recent(self, timestamp: datetime) -> bool:
        """Check if a timestamp is recent (within the last day)"""
        return timestamp >= self.yesterday

class DataEnricher:
    """Enricher for financial data"""
    
    def enrich_batch(self, batch: MarketBatch) -> MarketBatch:
        """
        Enrich a batch of financial data with additional information.
        This could include:
        - Adding sector information to stocks
        - Computing moving averages
        - Adding market cap data
        - etc.
        
        For now, this is a placeholder.
        """
        logger.info(f"Enriching batch {batch.batch_id}")
        
        # In a real implementation, we would:
        # 1. Look up additional data from databases/services
        # 2. Compute derived metrics
        # 3. Add the enriched data to the objects
        
        return batch
    
    def compute_statistics(self, batch: MarketBatch) -> Dict[str, Any]:
        """
        Compute statistical information about the batch
        """
        if not batch.quotes:
            return {}
        
        # Convert to DataFrame for easier analytics
        quotes_df = pd.DataFrame([quote.dict() for quote in batch.quotes])
        
        stats = {
            "mean_price": quotes_df["price"].mean(),
            "median_price": quotes_df["price"].median(),
            "mean_change_percent": quotes_df["change_percent"].mean(),
            "positive_change_count": (quotes_df["change"] > 0).sum(),
            "negative_change_count": (quotes_df["change"] < 0).sum(),
            "unchanged_count": (quotes_df["change"] == 0).sum(),
            "total_volume": quotes_df["volume"].sum(),
        }
        
        logger.info(f"Computed statistics for batch {batch.batch_id}")
        return stats