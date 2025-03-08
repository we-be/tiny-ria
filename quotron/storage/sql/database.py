#!/usr/bin/env python3
"""
Database access module for Quotron storage
"""

import os
import logging
import psycopg2
import psycopg2.extras
import psycopg2.pool
import contextlib
from typing import Dict, List, Any, Optional, Tuple, Generator

# Configure logging
logging.basicConfig(
    level=logging.INFO,
    format="%(asctime)s - %(name)s - %(levelname)s - %(message)s",
)
logger = logging.getLogger(__name__)

# Database connection parameters
DB_HOST = os.environ.get("DB_HOST", "localhost")
DB_PORT = int(os.environ.get("DB_PORT", "5432"))
DB_NAME = os.environ.get("DB_NAME", "quotron")
DB_USER = os.environ.get("DB_USER", "quotron")
DB_PASS = os.environ.get("DB_PASS", "quotron")
DB_POOL_MIN = int(os.environ.get("DB_POOL_MIN", "1"))
DB_POOL_MAX = int(os.environ.get("DB_POOL_MAX", "10"))

class Database:
    """Database access class for Quotron financial data"""
    
    _instance = None
    
    @classmethod
    def get_instance(cls):
        """Get the singleton instance of the database"""
        if cls._instance is None:
            cls._instance = cls()
        return cls._instance
    
    def __init__(self):
        """Initialize the database connection pool"""
        self.pool = psycopg2.pool.ThreadedConnectionPool(
            DB_POOL_MIN,
            DB_POOL_MAX,
            host=DB_HOST,
            port=DB_PORT,
            dbname=DB_NAME,
            user=DB_USER,
            password=DB_PASS
        )
        logger.info(f"Initialized database connection pool to {DB_HOST}:{DB_PORT}/{DB_NAME}")
    
    @contextlib.contextmanager
    def get_cursor(self):
        """Get a database cursor from the connection pool"""
        conn = self.pool.getconn()
        try:
            with conn:
                with conn.cursor(cursor_factory=psycopg2.extras.RealDictCursor) as cursor:
                    yield cursor
        finally:
            self.pool.putconn(conn)
    
    def insert_stock_quote(self, quote: Dict[str, Any]) -> str:
        """
        Insert a stock quote into the database
        Returns the ID of the inserted quote
        """
        with self.get_cursor() as cursor:
            cursor.execute(
                """
                INSERT INTO stock_quotes (
                    symbol, price, change, change_percent, volume, 
                    timestamp, exchange, source, batch_id
                ) VALUES (
                    %(symbol)s, %(price)s, %(change)s, %(change_percent)s, %(volume)s,
                    %(timestamp)s, %(exchange)s, %(source)s, %(batch_id)s
                ) RETURNING id
                """,
                {
                    "symbol": quote["symbol"],
                    "price": quote["price"],
                    "change": quote["change"],
                    "change_percent": quote["change_percent"],
                    "volume": quote["volume"],
                    "timestamp": quote["timestamp"],
                    "exchange": quote["exchange"],
                    "source": quote["source"],
                    "batch_id": quote.get("batch_id")
                }
            )
            result = cursor.fetchone()
            return result["id"]
    
    def insert_market_index(self, index: Dict[str, Any]) -> str:
        """
        Insert a market index into the database
        Returns the ID of the inserted index
        """
        with self.get_cursor() as cursor:
            cursor.execute(
                """
                INSERT INTO market_indices (
                    name, value, change, change_percent, timestamp, source, batch_id
                ) VALUES (
                    %(name)s, %(value)s, %(change)s, %(change_percent)s, 
                    %(timestamp)s, %(source)s, %(batch_id)s
                ) RETURNING id
                """,
                {
                    "name": index["name"],
                    "value": index["value"],
                    "change": index["change"],
                    "change_percent": index["change_percent"],
                    "timestamp": index["timestamp"],
                    "source": index["source"],
                    "batch_id": index.get("batch_id")
                }
            )
            result = cursor.fetchone()
            return result["id"]
    
    def insert_data_batch(self, batch: Dict[str, Any]) -> str:
        """
        Insert a data batch record into the database
        Returns the ID of the inserted batch
        """
        with self.get_cursor() as cursor:
            cursor.execute(
                """
                INSERT INTO data_batches (
                    id, created_at, status, quote_count, index_count, source, metadata
                ) VALUES (
                    %(id)s, %(created_at)s, %(status)s, %(quote_count)s, 
                    %(index_count)s, %(source)s, %(metadata)s
                ) RETURNING id
                """,
                {
                    "id": batch["id"],
                    "created_at": batch["created_at"],
                    "status": batch.get("status", "created"),
                    "quote_count": batch.get("quote_count", 0),
                    "index_count": batch.get("index_count", 0),
                    "source": batch["source"],
                    "metadata": psycopg2.extras.Json(batch.get("metadata", {}))
                }
            )
            result = cursor.fetchone()
            return result["id"]
    
    def update_batch_status(self, batch_id: str, status: str, processed_at=None) -> bool:
        """
        Update the status of a data batch
        Returns True if successful, False otherwise
        """
        with self.get_cursor() as cursor:
            cursor.execute(
                """
                UPDATE data_batches 
                SET status = %s, processed_at = COALESCE(%s, processed_at)
                WHERE id = %s
                """,
                (status, processed_at, batch_id)
            )
            return cursor.rowcount > 0
    
    def insert_batch_statistics(self, stats: Dict[str, Any]) -> str:
        """
        Insert batch statistics into the database
        Returns the ID of the inserted statistics record
        """
        with self.get_cursor() as cursor:
            cursor.execute(
                """
                INSERT INTO batch_statistics (
                    batch_id, mean_price, median_price, mean_change_percent,
                    positive_change_count, negative_change_count, unchanged_count,
                    total_volume, statistics_json
                ) VALUES (
                    %(batch_id)s, %(mean_price)s, %(median_price)s, %(mean_change_percent)s,
                    %(positive_change_count)s, %(negative_change_count)s, %(unchanged_count)s,
                    %(total_volume)s, %(statistics_json)s
                ) RETURNING id
                """,
                {
                    "batch_id": stats["batch_id"],
                    "mean_price": stats.get("mean_price"),
                    "median_price": stats.get("median_price"),
                    "mean_change_percent": stats.get("mean_change_percent"),
                    "positive_change_count": stats.get("positive_change_count"),
                    "negative_change_count": stats.get("negative_change_count"),
                    "unchanged_count": stats.get("unchanged_count"),
                    "total_volume": stats.get("total_volume"),
                    "statistics_json": psycopg2.extras.Json(stats.get("statistics_json", {}))
                }
            )
            result = cursor.fetchone()
            return result["id"]
    
    def get_latest_quotes(self, symbols: List[str] = None) -> List[Dict[str, Any]]:
        """
        Get the latest quotes for the specified symbols
        If symbols is None, get all the latest quotes
        """
        with self.get_cursor() as cursor:
            if symbols:
                placeholders = ', '.join(['%s'] * len(symbols))
                cursor.execute(
                    f"SELECT * FROM latest_stock_prices WHERE symbol IN ({placeholders})",
                    symbols
                )
            else:
                cursor.execute("SELECT * FROM latest_stock_prices")
            
            return cursor.fetchall()
    
    def get_latest_indices(self, names: List[str] = None) -> List[Dict[str, Any]]:
        """
        Get the latest indices for the specified names
        If names is None, get all the latest indices
        """
        with self.get_cursor() as cursor:
            if names:
                placeholders = ', '.join(['%s'] * len(names))
                cursor.execute(
                    f"SELECT * FROM latest_market_indices WHERE name IN ({placeholders})",
                    names
                )
            else:
                cursor.execute("SELECT * FROM latest_market_indices")
            
            return cursor.fetchall()
    
    def get_quotes_history(self, symbol: str, limit: int = 100) -> List[Dict[str, Any]]:
        """
        Get historical quotes for a specific symbol
        """
        with self.get_cursor() as cursor:
            cursor.execute(
                """
                SELECT * FROM stock_quotes
                WHERE symbol = %s
                ORDER BY timestamp DESC
                LIMIT %s
                """,
                (symbol, limit)
            )
            return cursor.fetchall()
    
    def get_index_history(self, name: str, limit: int = 100) -> List[Dict[str, Any]]:
        """
        Get historical values for a specific market index
        """
        with self.get_cursor() as cursor:
            cursor.execute(
                """
                SELECT * FROM market_indices
                WHERE name = %s
                ORDER BY timestamp DESC
                LIMIT %s
                """,
                (name, limit)
            )
            return cursor.fetchall()
    
    def get_batch(self, batch_id: str) -> Dict[str, Any]:
        """
        Get a specific data batch by ID
        """
        with self.get_cursor() as cursor:
            cursor.execute(
                "SELECT * FROM data_batches WHERE id = %s",
                (batch_id,)
            )
            return cursor.fetchone()
    
    def get_batch_quotes(self, batch_id: str) -> List[Dict[str, Any]]:
        """
        Get all stock quotes in a specific batch
        """
        with self.get_cursor() as cursor:
            cursor.execute(
                "SELECT * FROM stock_quotes WHERE batch_id = %s",
                (batch_id,)
            )
            return cursor.fetchall()
    
    def get_batch_indices(self, batch_id: str) -> List[Dict[str, Any]]:
        """
        Get all market indices in a specific batch
        """
        with self.get_cursor() as cursor:
            cursor.execute(
                "SELECT * FROM market_indices WHERE batch_id = %s",
                (batch_id,)
            )
            return cursor.fetchall()
    
    def get_batch_statistics(self, batch_id: str) -> Dict[str, Any]:
        """
        Get statistics for a specific batch
        """
        with self.get_cursor() as cursor:
            cursor.execute(
                "SELECT * FROM batch_statistics WHERE batch_id = %s",
                (batch_id,)
            )
            return cursor.fetchone()
    
    def close(self):
        """Close the database connection pool"""
        if self.pool:
            self.pool.closeall()
            logger.info("Closed database connection pool")
            self.pool = None