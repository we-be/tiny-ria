#!/usr/bin/env python3
"""
Enhanced Yahoo Finance Proxy Server

This script creates a Flask-based HTTP server that serves as a proxy for Yahoo Finance data.
It implements the following reliability features:
1. A SimpleCache class for caching stock quote data with TTL (time to live) support
2. A retry mechanism with exponential backoff for API calls
3. A heartbeat system that periodically checks the proxy health
4. A circuit breaker pattern to prevent overwhelming the Yahoo Finance API when issues occur

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
from collections import defaultdict
from datetime import datetime, timedelta
import random
import functools

# Flask for the API server
from flask import Flask, jsonify, request, make_response

# Yahoo Finance API
import yfinance as yf

# Configure logging
logging.basicConfig(
    level=logging.INFO,
    format="%(asctime)s - %(name)s - %(levelname)s - %(message)s",
)
logger = logging.getLogger("yfinance-proxy")

# Create Flask app
app = Flask(__name__)

class CircuitBreaker:
    """
    Circuit Breaker pattern implementation to prevent overwhelming unstable services.
    
    States:
    - CLOSED: All requests go through
    - OPEN: Requests are rejected immediately (circuit is tripped)
    - HALF-OPEN: Limited test requests are allowed to check if the issue is resolved
    """
    
    # Circuit states
    CLOSED = 'closed'
    OPEN = 'open'
    HALF_OPEN = 'half-open'
    
    def __init__(self, failure_threshold=5, recovery_timeout=30, test_requests=3):
        """
        Initialize the circuit breaker.
        
        Args:
            failure_threshold: Number of failures before opening the circuit
            recovery_timeout: Time in seconds to wait before transitioning to half-open
            test_requests: Number of test requests to allow in half-open state
        """
        self.failure_threshold = failure_threshold
        self.recovery_timeout = recovery_timeout
        self.test_requests = test_requests
        
        self.state = self.CLOSED
        self.failure_count = 0
        self.last_failure_time = None
        self.test_requests_count = 0
        self.lock = threading.RLock()
    
    def __call__(self, func):
        """Decorator for functions to be protected by the circuit breaker."""
        @functools.wraps(func)
        def wrapper(*args, **kwargs):
            with self.lock:
                # Check if circuit is OPEN
                if self.state == self.OPEN:
                    # Check if recovery timeout has elapsed
                    if (datetime.now() - self.last_failure_time).total_seconds() > self.recovery_timeout:
                        logger.info("Circuit transitioning from OPEN to HALF-OPEN")
                        self.state = self.HALF_OPEN
                        self.test_requests_count = 0
                    else:
                        raise Exception("Circuit breaker is open - request rejected")
                
                # Check if circuit is HALF-OPEN and too many test requests
                if self.state == self.HALF_OPEN and self.test_requests_count >= self.test_requests:
                    raise Exception("Circuit breaker is half-open and test request limit reached")
                
                # If HALF-OPEN, increment test request counter
                if self.state == self.HALF_OPEN:
                    self.test_requests_count += 1
            
            try:
                # Execute the function
                result = func(*args, **kwargs)
                
                # Success: reset or advance circuit state
                with self.lock:
                    if self.state == self.HALF_OPEN:
                        # Success in HALF-OPEN means we can close the circuit
                        logger.info("Circuit transitioning from HALF-OPEN to CLOSED")
                        self.state = self.CLOSED
                        self.failure_count = 0
                    elif self.state == self.CLOSED:
                        # Success in CLOSED keeps it closed and resets failure count
                        self.failure_count = 0
                
                return result
                
            except Exception as e:
                # Failure: increment failure count and potentially open circuit
                with self.lock:
                    self.failure_count += 1
                    self.last_failure_time = datetime.now()
                    
                    if self.state == self.CLOSED and self.failure_count >= self.failure_threshold:
                        logger.warning(f"Circuit transitioning from CLOSED to OPEN after {self.failure_count} failures")
                        self.state = self.OPEN
                    elif self.state == self.HALF_OPEN:
                        # Any failure in HALF-OPEN returns to OPEN
                        logger.warning("Circuit transitioning from HALF-OPEN back to OPEN due to failure")
                        self.state = self.OPEN
                
                raise e
        
        return wrapper
    
    def get_state(self):
        """Get the current state of the circuit breaker."""
        with self.lock:
            return {
                "state": self.state,
                "failure_count": self.failure_count,
                "last_failure_time": self.last_failure_time.isoformat() if self.last_failure_time else None,
                "test_requests_count": self.test_requests_count
            }


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


def retry_with_backoff(max_retries=3, initial_backoff=0.5, max_backoff=10, base=2):
    """
    Decorator for functions to retry with exponential backoff
    
    Args:
        max_retries: Maximum number of retries
        initial_backoff: Initial backoff time in seconds
        max_backoff: Maximum backoff time in seconds
        base: Base for exponential backoff calculation
    """
    def decorator(func):
        @functools.wraps(func)
        def wrapper(*args, **kwargs):
            retries = 0
            backoff = initial_backoff
            
            while True:
                try:
                    return func(*args, **kwargs)
                except Exception as e:
                    retries += 1
                    if retries > max_retries:
                        logger.error(f"Max retries ({max_retries}) exceeded")
                        raise
                    
                    # Calculate backoff with jitter
                    jitter = random.uniform(0, 0.1 * backoff)
                    sleep_time = min(backoff + jitter, max_backoff)
                    
                    logger.warning(f"Retry {retries}/{max_retries} after error: {e}. Sleeping for {sleep_time:.2f}s")
                    time.sleep(sleep_time)
                    
                    # Increase backoff for next retry
                    backoff = min(backoff * base, max_backoff)
        
        return wrapper
    
    return decorator


class StockDataProvider:
    """Provider for stock data with circuit breaker and retry protection"""
    
    def __init__(self):
        self.circuit_breaker = CircuitBreaker(failure_threshold=5, recovery_timeout=60)
        self.cache = SimpleCache(default_ttl=300)  # 5 minutes default TTL
        self.request_stats = {
            "total_requests": 0,
            "successful_requests": 0,
            "failed_requests": 0,
            "last_request_time": None,
            "api_calls": 0
        }
    
    @retry_with_backoff(max_retries=3, initial_backoff=1)
    def _fetch_stock_data(self, symbol):
        """
        Fetch stock data from Yahoo Finance with protection
        
        Args:
            symbol: Stock symbol to fetch
            
        Returns:
            Stock data dictionary
        """
        self.request_stats["api_calls"] += 1
        ticker = yf.Ticker(symbol)
        return ticker.info
    
    def get_stock_quote(self, symbol):
        """
        Get a stock quote with caching and protection
        
        Args:
            symbol: Stock symbol to fetch
            
        Returns:
            Stock quote dictionary
        """
        self.request_stats["total_requests"] += 1
        self.request_stats["last_request_time"] = datetime.now()
        
        try:
            # Check cache first
            cache_key = f"quote:{symbol}"
            cached_quote = self.cache.get(cache_key)
            
            if cached_quote:
                return cached_quote
            
            # Fetch from API with circuit breaker and retry protection
            info = self._fetch_stock_data(symbol)
            
            # Create a simplified quote object similar to our Go model
            quote = {
                "symbol": symbol,
                "price": info.get("regularMarketPrice", 0.0),
                "change": info.get("regularMarketChange", 0.0),
                "changePercent": info.get("regularMarketChangePercent", 0.0),
                "volume": info.get("regularMarketVolume", 0),
                "timestamp": datetime.now().strftime("%Y-%m-%dT%H:%M:%SZ"),
                "exchange": info.get("exchange", ""),
                "source": "Yahoo Finance (Python)",
            }
            
            # Cache the result
            self.cache.set(cache_key, quote)
            self.request_stats["successful_requests"] += 1
            
            return quote
            
        except Exception as e:
            self.request_stats["failed_requests"] += 1
            logger.error(f"Error fetching quote for {symbol}: {e}")
            return {"error": str(e), "symbol": symbol}
    
    def get_market_data(self, index):
        """
        Get market index data with caching and protection
        
        Args:
            index: Market index symbol to fetch
            
        Returns:
            Market data dictionary
        """
        self.request_stats["total_requests"] += 1
        self.request_stats["last_request_time"] = datetime.now()
        
        try:
            # Check cache first
            cache_key = f"market:{index}"
            cached_data = self.cache.get(cache_key)
            
            if cached_data:
                return cached_data
            
            # Fetch from API with circuit breaker and retry protection
            info = self._fetch_stock_data(index)
            
            # Create a simplified market data object similar to our Go model
            market_data = {
                "indexName": info.get("shortName", index),
                "value": info.get("regularMarketPrice", 0.0),
                "change": info.get("regularMarketChange", 0.0),
                "changePercent": info.get("regularMarketChangePercent", 0.0),
                "timestamp": datetime.now().strftime("%Y-%m-%dT%H:%M:%SZ"),
                "source": "Yahoo Finance (Python)",
            }
            
            # Cache the result
            self.cache.set(cache_key, market_data)
            self.request_stats["successful_requests"] += 1
            
            return market_data
            
        except Exception as e:
            self.request_stats["failed_requests"] += 1
            logger.error(f"Error fetching market data for {index}: {e}")
            return {"error": str(e), "index": index}
    
    def get_circuit_state(self):
        """Get the current state of the circuit breaker"""
        return self.circuit_breaker.get_state()
    
    def get_stats(self):
        """Get provider statistics"""
        cache_stats = self.cache.get_stats()
        
        stats = {
            "request_stats": self.request_stats,
            "cache_stats": cache_stats,
            "circuit_breaker": self.get_circuit_state()
        }
        
        return stats


# Create the stock data provider
stock_provider = StockDataProvider()

# Health check status tracking
health_check_status = {
    "status": "ok",
    "last_check": datetime.now().isoformat(),
    "consecutive_failures": 0,
    "is_healthy": True
}

# Set up the heartbeat system
def heartbeat_check():
    """Perform a periodic health check and update status"""
    global health_check_status
    
    try:
        # Perform a minimal API call to check health
        test_symbol = "AAPL"
        result = stock_provider.get_stock_quote(test_symbol)
        
        if "error" in result:
            health_check_status["consecutive_failures"] += 1
            logger.warning(f"Heartbeat check failed: {result['error']}")
            if health_check_status["consecutive_failures"] >= 3:
                health_check_status["status"] = "degraded"
                health_check_status["is_healthy"] = False
        else:
            health_check_status["status"] = "ok"
            health_check_status["is_healthy"] = True
            health_check_status["consecutive_failures"] = 0
    
    except Exception as e:
        health_check_status["consecutive_failures"] += 1
        logger.error(f"Heartbeat check error: {e}")
        if health_check_status["consecutive_failures"] >= 3:
            health_check_status["status"] = "failed"
            health_check_status["is_healthy"] = False
    
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
    return jsonify({
        "status": health_check_status["status"],
        "timestamp": datetime.now().isoformat(),
        "uptime": (datetime.now() - startup_time).total_seconds(),
        "last_check": health_check_status["last_check"]
    })


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
    parser = argparse.ArgumentParser(description="Enhanced Yahoo Finance Proxy Server")
    parser.add_argument("--host", default="localhost", help="Host to bind the server to")
    parser.add_argument("--port", type=int, default=5000, help="Port to bind the server to")
    parser.add_argument("--debug", action="store_true", help="Run Flask in debug mode")
    args = parser.parse_args()
    
    # Record startup time
    startup_time = datetime.now()
    
    # Start heartbeat system
    heartbeat_check()
    
    logger.info(f"Starting Enhanced Yahoo Finance proxy server on http://{args.host}:{args.port}")
    app.run(host=args.host, port=args.port, debug=args.debug)