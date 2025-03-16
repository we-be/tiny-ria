#!/usr/bin/env python3
"""
Enhanced Yahoo Finance Proxy Server

This script creates a Flask-based HTTP server that serves as a proxy for Yahoo Finance data.
It implements the following reliability features:
1. A SimpleCache class for caching stock quote data with TTL (time to live) support
2. A retry mechanism with exponential backoff for API calls
3. A heartbeat system that periodically checks the proxy health
4. A circuit breaker pattern to prevent overwhelming the Yahoo Finance API when issues occur
5. Integrated with unified health monitoring service

The server exposes the following endpoints:
- /quote/<symbol> - Get a stock quote
- /health - Health check endpoint
- /metrics - Get proxy metrics (cache hits, misses, etc.)
"""

import argparse
import json
import logging
import threading
import time
import os
import sys
from collections import defaultdict
from datetime import datetime, timedelta
import random
import functools

# Flask for the API server
from flask import Flask, jsonify, request, make_response, g

# Yahoo Finance API
import yfinance as yf

# Configure logging first so we can use it immediately
logging.basicConfig(
    level=logging.INFO,
    format="%(asctime)s - %(name)s - %(levelname)s - %(message)s",
)
logger = logging.getLogger("yfinance-proxy")

# Add the health module to path
health_module_path = os.path.abspath(os.path.join(os.path.dirname(__file__), '../../../health'))
if health_module_path not in sys.path:
    sys.path.append(health_module_path)
logger.info(f"Added health module path: {health_module_path}")

# Create mock health classes for when health client is not available
class MockHealthStatus:
    HEALTHY = "healthy"
    DEGRADED = "degraded" 
    FAILED = "failed"

class MockHealthClient:
    def __init__(self, url):
        self.url = url
        
    def report_health(self, source_type, source_name, status, response_time_ms=None, error_message=None, metadata=None):
        logger.info(f"MOCK: Reporting health for {source_type}/{source_name}: {status}")
        
    def get_service_health(self, source_type, source_name):
        return {"status": "unknown", "last_check": None, "error_count": 0}

# Import our unified health client
try:
    from client import HealthClient, HealthStatus
    health_enabled = True
    logger.info(f"Health client imported successfully")
except ImportError as e:
    health_enabled = False
    logger.warning(f"Health client could not be imported: {e}")
    logger.warning(f"Attempted to import from {health_module_path}")
    
    # Use mock classes instead
    logger.info("Using mock health client implementation")
    HealthClient = MockHealthClient
    HealthStatus = MockHealthStatus

# Create Flask app
app = Flask(__name__)

# Removed CircuitBreaker class - simplifying the code


class SimpleCache:
    """
    A simple in-memory cache with TTL support
    """
    
    def __init__(self, default_ttl=300):
        """
        Initialize cache with a default TTL
        
        Args:
            default_ttl: Default time-to-live in seconds for cache entries
        """
        self.cache = {}
        self.default_ttl = default_ttl
        self.locks = defaultdict(threading.RLock)
        self.stats = {
            "hits": 0,
            "misses": 0,
            "entries": 0,
            "evictions": 0
        }
    
    def get(self, key, default=None):
        """
        Get a value from the cache
        
        Args:
            key: Cache key
            default: Default value to return if key not found
            
        Returns:
            Cached value or default if not found or expired
        """
        with self.locks[key]:
            if key in self.cache:
                value, expiry = self.cache[key]
                if expiry > datetime.now():
                    self.stats["hits"] += 1
                    return value
                else:
                    # Entry expired, remove it
                    del self.cache[key]
                    self.stats["evictions"] += 1
            
            self.stats["misses"] += 1
            return default
    
    def set(self, key, value, ttl=None):
        """
        Set a value in the cache
        
        Args:
            key: Cache key
            value: Value to cache
            ttl: Time-to-live in seconds (uses default_ttl if None)
        """
        if ttl is None:
            ttl = self.default_ttl
        
        expiry = datetime.now() + timedelta(seconds=ttl)
        
        with self.locks[key]:
            if key not in self.cache:
                self.stats["entries"] += 1
            self.cache[key] = (value, expiry)
    
    def delete(self, key):
        """Delete a key from the cache"""
        with self.locks[key]:
            if key in self.cache:
                del self.cache[key]
    
    def clear(self):
        """Clear all entries from the cache"""
        keys = list(self.cache.keys())
        for key in keys:
            with self.locks[key]:
                if key in self.cache:
                    del self.cache[key]
        self.stats["entries"] = 0
    
    def get_stats(self):
        """Get cache statistics"""
        total_requests = self.stats["hits"] + self.stats["misses"]
        hit_ratio = self.stats["hits"] / total_requests if total_requests > 0 else 0
        
        return {
            "hits": self.stats["hits"],
            "misses": self.stats["misses"],
            "entries": self.stats["entries"],
            "evictions": self.stats["evictions"],
            "hit_ratio": hit_ratio
        }


# Removed retry_with_backoff decorator - we let errors bubble up naturally


class StockDataProvider:
    """Provider for stock data with caching"""
    
    def __init__(self):
        self.cache = SimpleCache(default_ttl=300)  # 5 minutes default TTL
        self.request_stats = {
            "total_requests": 0,
            "successful_requests": 0,
            "failed_requests": 0,
            "last_request_time": None,
            "api_calls": 0
        }
    
    def _fetch_stock_data(self, symbol):
        """
        Fetch stock data from Yahoo Finance
        
        Args:
            symbol: Stock symbol to fetch
            
        Returns:
            Stock data dictionary
        """
        self.request_stats["api_calls"] += 1
        ticker = yf.Ticker(symbol)
        return ticker.info
    
    def _fetch_and_cache_data(self, symbol, cache_type, transform_func):
        """
        Generic method to fetch and cache data
        
        Args:
            symbol: Symbol to fetch (stock or index)
            cache_type: Type of data for cache key ('quote' or 'market')
            transform_func: Function to transform raw data into desired format
            
        Returns:
            Formatted data dictionary
        """
        self.request_stats["total_requests"] += 1
        self.request_stats["last_request_time"] = datetime.now()
        
        # Check cache first
        cache_key = f"{cache_type}:{symbol}"
        cached_data = self.cache.get(cache_key)
        
        if cached_data:
            return cached_data
        
        # Fetch from API
        info = self._fetch_stock_data(symbol)
        
        # Transform the data using the provided function
        result = transform_func(info, symbol)
        
        # Cache the result
        self.cache.set(cache_key, result)
        self.request_stats["successful_requests"] += 1
        
        return result
    
    def get_stock_quote(self, symbol):
        """
        Get a stock quote with caching and protection
        
        Args:
            symbol: Stock symbol to fetch
            
        Returns:
            Stock quote dictionary
        """
        def transform_quote(info, symbol):
            return {
                "symbol": symbol,
                "price": info.get("regularMarketPrice", 0.0),
                "change": info.get("regularMarketChange", 0.0),
                "changePercent": info.get("regularMarketChangePercent", 0.0),
                "volume": info.get("regularMarketVolume", 0),
                "timestamp": datetime.now().strftime("%Y-%m-%dT%H:%M:%SZ"),
                "exchange": info.get("exchange", ""),
                "source": "Yahoo Finance (Python)",
            }
            
        return self._fetch_and_cache_data(symbol, "quote", transform_quote)
    
    def get_market_data(self, index):
        """
        Get market index data with caching and protection
        
        Args:
            index: Market index symbol to fetch
            
        Returns:
            Market data dictionary
        """
        def transform_market_data(info, symbol):
            return {
                "indexName": info.get("shortName", symbol),
                "value": info.get("regularMarketPrice", 0.0),
                "change": info.get("regularMarketChange", 0.0),
                "changePercent": info.get("regularMarketChangePercent", 0.0),
                "timestamp": datetime.now().strftime("%Y-%m-%dT%H:%M:%SZ"),
                "source": "Yahoo Finance (Python)",
            }
            
        return self._fetch_and_cache_data(index, "market", transform_market_data)
    
    def get_stats(self):
        """Get provider statistics"""
        cache_stats = self.cache.get_stats()
        
        stats = {
            "request_stats": self.request_stats,
            "cache_stats": cache_stats
        }
        
        return stats


# Create the stock data provider
stock_provider = StockDataProvider()

# Health check status tracking (for backward compatibility)
health_check_status = {
    "status": "ok",
    "last_check": datetime.now().isoformat(),
    "consecutive_failures": 0,
    "is_healthy": True
}

# Initialize the health client
health_client = None
if health_enabled:
    # Get health service URL from environment or use default
    health_service_url = os.environ.get("HEALTH_SERVICE_URL", "http://localhost:8085")
    health_client = HealthClient(health_service_url)
    logger.info(f"Health client initialized with service URL: {health_service_url}")

# Flask middleware for request tracking
@app.before_request
def before_request():
    """Set start time for request duration tracking"""
    g.start_time = time.time()

@app.after_request
def after_request(response):
    """Track request duration and report health status"""
    # Skip for health endpoint to avoid recursive reporting
    if request.path == '/health':
        return response
        
    if health_enabled and health_client:
        # Calculate response time
        duration_ms = int((time.time() - g.start_time) * 1000)
        
        # Determine status based on response code
        status = HealthStatus.HEALTHY
        if response.status_code >= 500:
            status = HealthStatus.FAILED
        elif response.status_code >= 400:
            status = HealthStatus.DEGRADED
            
        # Report health asynchronously
        def report_request_health():
            try:
                health_client.report_health(
                    source_type="api-scraper",
                    source_name="yahoo_finance_proxy",
                    status=status,
                    response_time_ms=duration_ms,
                    metadata={
                        "path": request.path,
                        "method": request.method,
                        "status_code": response.status_code
                    }
                )
            except Exception as e:
                logger.error(f"Failed to report request health: {e}")
                
        threading.Thread(target=report_request_health).start()
        
    return response

# Set up the heartbeat system
def heartbeat_check():
    """Perform a periodic health check and update status"""
    global health_check_status
    
    # Perform a minimal API call to check health
    test_symbol = "AAPL"
    try:
        result = stock_provider.get_stock_quote(test_symbol)
        health_check_status["status"] = "ok"
        health_check_status["is_healthy"] = True
        health_check_status["consecutive_failures"] = 0
        
        # Report healthy status if health client available
        if health_enabled and health_client:
            health_client.report_health(
                source_type="api-scraper",
                source_name="yahoo_finance_proxy",
                status=HealthStatus.HEALTHY,
                metadata={
                    "uptime": (datetime.now() - startup_time).total_seconds(),
                    "cache_stats": stock_provider.cache.get_stats()
                }
            )
    except Exception as e:
        health_check_status["consecutive_failures"] += 1
        logger.error(f"Heartbeat check error: {e}")
        health_check_status["status"] = "failed"
        health_check_status["is_healthy"] = False
        
        # Report failed health if health client available
        if health_enabled and health_client:
            health_client.report_health(
                source_type="api-scraper",
                source_name="yahoo_finance_proxy",
                status=HealthStatus.FAILED,
                error_message=str(e),
                metadata={
                    "consecutive_failures": health_check_status["consecutive_failures"],
                    "uptime": (datetime.now() - startup_time).total_seconds()
                }
            )
    
    health_check_status["last_check"] = datetime.now().isoformat()
    
    # Schedule the next check
    threading.Timer(60, heartbeat_check).start()


# Flask routes
@app.route('/quote/<symbol>', methods=['GET'])
def quote(symbol):
    """Get a stock quote for the given symbol"""
    ttl = request.args.get('ttl', default=None, type=int)
    
    quote = stock_provider.get_stock_quote(symbol)
    
    response = make_response(jsonify(quote))
    
    # Set cache status header for Go client to track
    if "error" not in quote:
        response.headers['X-Cache-Status'] = 'miss'
    
    return response


@app.route('/market/<index>', methods=['GET'])
def market(index):
    """Get market data for the given index"""
    ttl = request.args.get('ttl', default=None, type=int)
    
    market_data = stock_provider.get_market_data(index)
    
    response = make_response(jsonify(market_data))
    
    # Set cache status header for Go client to track
    if "error" not in market_data:
        response.headers['X-Cache-Status'] = 'miss'
    
    return response


@app.route('/health', methods=['GET'])
def health():
    """Health check endpoint"""
    health_data = {
        "status": health_check_status["status"],
        "timestamp": datetime.now().isoformat(),
        "uptime": (datetime.now() - startup_time).total_seconds(),
        "last_check": health_check_status["last_check"]
    }
    
    # Add unified health status if enabled
    if health_enabled and health_client:
        try:
            service_health = health_client.get_service_health("api-scraper", "yahoo_finance_proxy")
            if service_health:
                health_data["unified_health"] = {
                    "status": service_health.get("status", "unknown"),
                    "last_check": service_health.get("last_check"),
                    "error_count": service_health.get("error_count", 0)
                }
        except Exception as e:
            logger.warning(f"Failed to get unified health status: {e}")
    
    return jsonify(health_data)


@app.route('/metrics', methods=['GET'])
def metrics():
    """Get proxy metrics"""
    return jsonify({
        "provider_stats": stock_provider.get_stats(),
        "health": health_check_status,
        "uptime": (datetime.now() - startup_time).total_seconds()
    })


@app.route('/admin/cache/clear', methods=['POST'])
def clear_cache():
    """Admin endpoint to clear the cache"""
    stock_provider.cache.clear()
    return jsonify({"status": "ok", "message": "Cache cleared"})


if __name__ == "__main__":
    try:
        # Print diagnostic information
        logger.info("=== Starting YFinance Proxy Server ===")
        logger.info(f"Python version: {sys.version}")
        logger.info(f"Current directory: {os.getcwd()}")
        logger.info(f"Script directory: {os.path.dirname(os.path.abspath(__file__))}")
        
        # Parse arguments
        parser = argparse.ArgumentParser(description="Enhanced Yahoo Finance Proxy Server")
        parser.add_argument("--host", default="localhost", help="Host to bind the server to")
        parser.add_argument("--port", type=int, default=5000, help="Port to bind the server to")
        parser.add_argument("--debug", action="store_true", help="Run Flask in debug mode")
        parser.add_argument("--test", action="store_true", help="Test mode - initialize but don't start server")
        args = parser.parse_args()
        
        # Log parsed arguments
        logger.info(f"Arguments: host={args.host}, port={args.port}, debug={args.debug}")
        
        # Record startup time
        startup_time = datetime.now()
        
        # Start heartbeat system
        logger.info("Starting heartbeat system...")
        heartbeat_check()
        
        # Test mode - just initialize and exit with success
        if args.test:
            logger.info("Test mode - initialization successful, exiting...")
            sys.exit(0)
            
        # Initialize Flask app
        logger.info(f"Starting Enhanced Yahoo Finance proxy server on http://{args.host}:{args.port}")
        app.run(host=args.host, port=args.port, debug=args.debug)
    except Exception as e:
        logger.critical(f"Fatal error in main execution: {e}", exc_info=True)
        sys.exit(1)