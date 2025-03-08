#!/usr/bin/env python3
"""
Command-line interface for Quotron data pipeline
"""

import os
import sys
import argparse
import logging
import json
import time
from datetime import datetime
from typing import Dict, List, Any, Optional

# Add parent directory to path for imports
sys.path.append(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))

from schemas.finance_schema import DataSource
from ingestor import DataIngestor
from storage.sql.db_manager import DBManager
from storage.sql.database import Database

# Configure logging
logging.basicConfig(
    level=logging.INFO,
    format="%(asctime)s - %(name)s - %(levelname)s - %(message)s",
)
logger = logging.getLogger(__name__)

def setup_database():
    """Set up the database schema"""
    logger.info("Setting up database schema...")
    db_manager = DBManager()
    db_manager.migrate_up()
    logger.info("Database schema setup complete")

def load_json_file(file_path: str) -> Any:
    """Load data from a JSON file"""
    logger.info(f"Loading data from {file_path}")
    with open(file_path, 'r') as f:
        return json.load(f)

def process_quotes_file(ingestor: DataIngestor, file_path: str, source: DataSource):
    """Process a file containing stock quotes"""
    data = load_json_file(file_path)
    
    if isinstance(data, list):
        # List of quotes
        batch_id, quote_ids = ingestor.process_stock_quotes(data, source)
        logger.info(f"Processed {len(quote_ids)} quotes in batch {batch_id}")
    elif isinstance(data, dict) and "quotes" in data:
        # Dictionary with a quotes key
        batch_id, quote_ids = ingestor.process_stock_quotes(data["quotes"], source)
        logger.info(f"Processed {len(quote_ids)} quotes in batch {batch_id}")
    else:
        logger.error(f"Unsupported data format in {file_path}")

def process_indices_file(ingestor: DataIngestor, file_path: str, source: DataSource):
    """Process a file containing market indices"""
    data = load_json_file(file_path)
    
    if isinstance(data, list):
        # List of indices
        batch_id, index_ids = ingestor.process_market_indices(data, source)
        logger.info(f"Processed {len(index_ids)} indices in batch {batch_id}")
    elif isinstance(data, dict) and "indices" in data:
        # Dictionary with an indices key
        batch_id, index_ids = ingestor.process_market_indices(data["indices"], source)
        logger.info(f"Processed {len(index_ids)} indices in batch {batch_id}")
    else:
        logger.error(f"Unsupported data format in {file_path}")

def process_mixed_file(ingestor: DataIngestor, file_path: str, source: DataSource):
    """Process a file containing both stock quotes and market indices"""
    data = load_json_file(file_path)
    
    quotes = []
    indices = []
    
    if isinstance(data, dict):
        # Extract quotes and indices from dict
        quotes = data.get("quotes", [])
        indices = data.get("indices", [])
    else:
        logger.error(f"Unsupported data format in {file_path}")
        return
    
    batch_id, quote_ids, index_ids = ingestor.process_mixed_batch(quotes, indices, source)
    logger.info(f"Processed {len(quote_ids)} quotes and {len(index_ids)} indices in batch {batch_id}")

def process_realtime_stream(ingestor: DataIngestor, source: DataSource, duration_seconds: int = 60):
    """Process a simulated real-time stream of data"""
    logger.info(f"Starting real-time processing from {source} for {duration_seconds} seconds")
    
    start_time = time.time()
    count = 0
    
    while time.time() - start_time < duration_seconds:
        # Simulate receiving a batch of quotes every few seconds
        batch_size = 5  # Small batch size for real-time
        
        # Generate simulated data (in a real system, this would come from a message queue)
        quotes = []
        indices = []
        
        # Add dummy data for illustration - this would be real data in production
        for i in range(batch_size):
            quotes.append({
                "symbol": f"SIM{i}",
                "price": 100 + i,
                "change": i,
                "change_percent": i,
                "volume": 1000 * i,
                "timestamp": datetime.utcnow().isoformat(),
                "exchange": "NYSE"
            })
        
        # Process the batch
        batch_id, quote_ids = ingestor.process_stock_quotes(quotes, source)
        count += 1
        
        logger.info(f"Processed real-time batch {count}: {batch_id} with {len(quote_ids)} quotes")
        
        # Sleep for a short time to simulate data arriving at intervals
        time.sleep(5)
    
    logger.info(f"Completed real-time processing: {count} batches processed")

def list_latest_data(db: Database, limit: int = 10):
    """List the latest stock quotes and market indices"""
    logger.info(f"Listing latest {limit} stock quotes")
    quotes = db.get_latest_quotes()
    for quote in quotes[:limit]:
        logger.info(f"Quote: {quote['symbol']} - ${quote['price']} ({quote['change_percent']}%)")
    
    logger.info(f"Listing latest {limit} market indices")
    indices = db.get_latest_indices()
    for index in indices[:limit]:
        logger.info(f"Index: {index['name']} - {index['value']} ({index['change_percent']}%)")

def main():
    """Main entry point"""
    parser = argparse.ArgumentParser(description="Quotron Data Pipeline CLI")
    subparsers = parser.add_subparsers(dest="command", help="Commands")
    
    # Setup command
    setup_parser = subparsers.add_parser("setup", help="Set up the database schema")
    
    # Process file commands
    quotes_parser = subparsers.add_parser("quotes", help="Process a file of stock quotes")
    quotes_parser.add_argument("file", help="Path to the JSON file containing stock quotes")
    quotes_parser.add_argument("--source", choices=["api-scraper", "browser-scraper", "manual"], 
                              default="manual", help="Source of the data")
    
    indices_parser = subparsers.add_parser("indices", help="Process a file of market indices")
    indices_parser.add_argument("file", help="Path to the JSON file containing market indices")
    indices_parser.add_argument("--source", choices=["api-scraper", "browser-scraper", "manual"], 
                               default="manual", help="Source of the data")
    
    mixed_parser = subparsers.add_parser("mixed", help="Process a file containing both quotes and indices")
    mixed_parser.add_argument("file", help="Path to the JSON file containing mixed data")
    mixed_parser.add_argument("--source", choices=["api-scraper", "browser-scraper", "manual"], 
                             default="manual", help="Source of the data")
    
    # Real-time processing command
    realtime_parser = subparsers.add_parser("realtime", help="Process simulated real-time data")
    realtime_parser.add_argument("--source", choices=["api-scraper", "browser-scraper", "manual"], 
                                default="api-scraper", help="Source of the data")
    realtime_parser.add_argument("--duration", type=int, default=60,
                                help="Duration in seconds to run the real-time processing")
    
    # List data command
    list_parser = subparsers.add_parser("list", help="List the latest data")
    list_parser.add_argument("--limit", type=int, default=10, help="Number of items to list")
    
    args = parser.parse_args()
    
    if args.command == "setup":
        setup_database()
    
    elif args.command == "quotes":
        ingestor = DataIngestor()
        process_quotes_file(ingestor, args.file, args.source)
        ingestor.close()
    
    elif args.command == "indices":
        ingestor = DataIngestor()
        process_indices_file(ingestor, args.file, args.source)
        ingestor.close()
    
    elif args.command == "mixed":
        ingestor = DataIngestor()
        process_mixed_file(ingestor, args.file, args.source)
        ingestor.close()
    
    elif args.command == "realtime":
        ingestor = DataIngestor()
        process_realtime_stream(ingestor, args.source, args.duration)
        ingestor.close()
    
    elif args.command == "list":
        db = Database.get_instance()
        list_latest_data(db, args.limit)
        db.close()
    
    else:
        parser.print_help()

if __name__ == "__main__":
    main()