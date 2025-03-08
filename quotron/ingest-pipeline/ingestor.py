#!/usr/bin/env python3
"""
Data ingestor for Quotron financial data pipeline
"""

import os
import sys
import logging
import json
import uuid
from datetime import datetime
import pandas as pd
from typing import Dict, List, Any, Optional, Tuple

# Add parent directory to path for imports
sys.path.append(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))

from schemas.finance_schema import StockQuote, MarketIndex, MarketBatch, DataSource
from validation.validators import DataValidator, DataEnricher
from storage.sql.database import Database

# Configure logging
logging.basicConfig(
    level=logging.INFO,
    format="%(asctime)s - %(name)s - %(levelname)s - %(message)s",
)
logger = logging.getLogger(__name__)

class DataIngestor:
    """
    Data ingestor for financial data pipeline
    
    This class is responsible for:
    1. Validating and cleaning incoming data
    2. Enriching data with additional information
    3. Storing data in the database
    4. Computing statistics on the data
    5. Handling errors and retries
    """
    
    def __init__(self):
        """Initialize the data ingestor"""
        self.validator = DataValidator()
        self.enricher = DataEnricher()
        self.db = Database.get_instance()
        logger.info("Initialized data ingestor")
    
    def process_stock_quotes(self, quotes_data: List[Dict[str, Any]], source: DataSource) -> Tuple[str, List[str]]:
        """
        Process a list of stock quotes
        Returns the batch ID and a list of successfully processed quote IDs
        """
        # Create a batch ID
        batch_id = f"quotes_{datetime.utcnow().strftime('%Y%m%d%H%M%S')}_{uuid.uuid4().hex[:8]}"
        logger.info(f"Processing batch {batch_id} with {len(quotes_data)} quotes from {source}")
        
        # Validate and enrich quotes
        valid_quotes = []
        for quote_data in quotes_data:
            # Add source and batch_id to the quote data
            quote_data["source"] = source
            quote_data["batch_id"] = batch_id
            
            # Validate the quote
            quote = self.validator.validate_stock_quote(quote_data)
            if quote:
                valid_quotes.append(quote)
        
        # Create a batch record
        batch_record = {
            "id": batch_id,
            "created_at": datetime.utcnow(),
            "status": "processing",
            "quote_count": len(valid_quotes),
            "index_count": 0,
            "source": source,
            "metadata": {"original_count": len(quotes_data)}
        }
        
        try:
            # Store the batch record
            self.db.insert_data_batch(batch_record)
            
            # Store each valid quote
            quote_ids = []
            for quote in valid_quotes:
                quote_id = self.db.insert_stock_quote(quote.dict())
                quote_ids.append(quote_id)
            
            # Compute and store statistics if we have valid quotes
            if valid_quotes:
                batch = MarketBatch(quotes=valid_quotes, indices=[], batch_id=batch_id)
                enriched_batch = self.enricher.enrich_batch(batch)
                stats = self.enricher.compute_statistics(enriched_batch)
                
                stats_record = {
                    "batch_id": batch_id,
                    "mean_price": stats.get("mean_price"),
                    "median_price": stats.get("median_price"),
                    "mean_change_percent": stats.get("mean_change_percent"),
                    "positive_change_count": stats.get("positive_change_count"),
                    "negative_change_count": stats.get("negative_change_count"),
                    "unchanged_count": stats.get("unchanged_count"),
                    "total_volume": stats.get("total_volume"),
                    "statistics_json": stats
                }
                self.db.insert_batch_statistics(stats_record)
            
            # Update the batch status to completed
            self.db.update_batch_status(batch_id, "completed", datetime.utcnow())
            logger.info(f"Successfully processed batch {batch_id} with {len(quote_ids)} quotes")
            
            return batch_id, quote_ids
            
        except Exception as e:
            # Update the batch status to failed
            self.db.update_batch_status(batch_id, "failed", datetime.utcnow())
            logger.error(f"Failed to process batch {batch_id}: {e}")
            raise
    
    def process_market_indices(self, indices_data: List[Dict[str, Any]], source: DataSource) -> Tuple[str, List[str]]:
        """
        Process a list of market indices
        Returns the batch ID and a list of successfully processed index IDs
        """
        # Create a batch ID
        batch_id = f"indices_{datetime.utcnow().strftime('%Y%m%d%H%M%S')}_{uuid.uuid4().hex[:8]}"
        logger.info(f"Processing batch {batch_id} with {len(indices_data)} indices from {source}")
        
        # Validate indices
        valid_indices = []
        for index_data in indices_data:
            # Add source and batch_id to the index data
            index_data["source"] = source
            index_data["batch_id"] = batch_id
            
            # Validate the index
            index = self.validator.validate_market_index(index_data)
            if index:
                valid_indices.append(index)
        
        # Create a batch record
        batch_record = {
            "id": batch_id,
            "created_at": datetime.utcnow(),
            "status": "processing",
            "quote_count": 0,
            "index_count": len(valid_indices),
            "source": source,
            "metadata": {"original_count": len(indices_data)}
        }
        
        try:
            # Store the batch record
            self.db.insert_data_batch(batch_record)
            
            # Store each valid index
            index_ids = []
            for index in valid_indices:
                index_id = self.db.insert_market_index(index.dict())
                index_ids.append(index_id)
            
            # Update the batch status to completed
            self.db.update_batch_status(batch_id, "completed", datetime.utcnow())
            logger.info(f"Successfully processed batch {batch_id} with {len(index_ids)} indices")
            
            return batch_id, index_ids
            
        except Exception as e:
            # Update the batch status to failed
            self.db.update_batch_status(batch_id, "failed", datetime.utcnow())
            logger.error(f"Failed to process batch {batch_id}: {e}")
            raise
    
    def process_mixed_batch(self, quotes_data: List[Dict[str, Any]], indices_data: List[Dict[str, Any]], source: DataSource) -> Tuple[str, List[str], List[str]]:
        """
        Process a mixed batch of stock quotes and market indices
        Returns the batch ID and lists of successfully processed quote and index IDs
        """
        # Create a batch ID
        batch_id = f"mixed_{datetime.utcnow().strftime('%Y%m%d%H%M%S')}_{uuid.uuid4().hex[:8]}"
        logger.info(f"Processing batch {batch_id} with {len(quotes_data)} quotes and {len(indices_data)} indices from {source}")
        
        # Validate and enrich data
        valid_quotes = []
        for quote_data in quotes_data:
            # Add source and batch_id to the quote data
            quote_data["source"] = source
            quote_data["batch_id"] = batch_id
            
            # Validate the quote
            quote = self.validator.validate_stock_quote(quote_data)
            if quote:
                valid_quotes.append(quote)
        
        valid_indices = []
        for index_data in indices_data:
            # Add source and batch_id to the index data
            index_data["source"] = source
            index_data["batch_id"] = batch_id
            
            # Validate the index
            index = self.validator.validate_market_index(index_data)
            if index:
                valid_indices.append(index)
        
        # Create a batch record
        batch_record = {
            "id": batch_id,
            "created_at": datetime.utcnow(),
            "status": "processing",
            "quote_count": len(valid_quotes),
            "index_count": len(valid_indices),
            "source": source,
            "metadata": {
                "original_quote_count": len(quotes_data),
                "original_index_count": len(indices_data)
            }
        }
        
        try:
            # Store the batch record
            self.db.insert_data_batch(batch_record)
            
            # Store each valid quote
            quote_ids = []
            for quote in valid_quotes:
                quote_id = self.db.insert_stock_quote(quote.dict())
                quote_ids.append(quote_id)
            
            # Store each valid index
            index_ids = []
            for index in valid_indices:
                index_id = self.db.insert_market_index(index.dict())
                index_ids.append(index_id)
            
            # Compute and store statistics if we have valid quotes
            if valid_quotes:
                batch = MarketBatch(quotes=valid_quotes, indices=valid_indices, batch_id=batch_id)
                enriched_batch = self.enricher.enrich_batch(batch)
                stats = self.enricher.compute_statistics(enriched_batch)
                
                stats_record = {
                    "batch_id": batch_id,
                    "mean_price": stats.get("mean_price"),
                    "median_price": stats.get("median_price"),
                    "mean_change_percent": stats.get("mean_change_percent"),
                    "positive_change_count": stats.get("positive_change_count"),
                    "negative_change_count": stats.get("negative_change_count"),
                    "unchanged_count": stats.get("unchanged_count"),
                    "total_volume": stats.get("total_volume"),
                    "statistics_json": stats
                }
                self.db.insert_batch_statistics(stats_record)
            
            # Update the batch status to completed
            self.db.update_batch_status(batch_id, "completed", datetime.utcnow())
            logger.info(f"Successfully processed batch {batch_id} with {len(quote_ids)} quotes and {len(index_ids)} indices")
            
            return batch_id, quote_ids, index_ids
            
        except Exception as e:
            # Update the batch status to failed
            self.db.update_batch_status(batch_id, "failed", datetime.utcnow())
            logger.error(f"Failed to process batch {batch_id}: {e}")
            raise
    
    def close(self):
        """Close connections"""
        if self.db:
            self.db.close()
            self.db = None