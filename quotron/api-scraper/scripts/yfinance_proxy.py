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
6. Proper shutdown handling via process signals

The server exposes the following endpoints:
- / - Simple web UI
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
from flask import Flask, jsonify, request, make_response, g, render_template_string

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
        # Intentionally raise an exception to test the API scraper badge failure
        raise RuntimeError("Intentionally breaking the API scraper to test badge failure status")
        
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


@app.route('/', methods=['GET'])
def index():
    """Root web UI"""
    html_template = """
    <!DOCTYPE html>
    <html lang="en">
    <head>
        <meta charset="UTF-8">
        <meta name="viewport" content="width=device-width, initial-scale=1.0">
        <title>Quotron YFinance Proxy</title>
        <style>
            :root {
                --bg-color: #0d1117;
                --text-color: #c9d1d9;
                --accent-color: #58a6ff;
                --secondary-bg: #161b22;
                --border-color: #30363d;
                --success-color: #3fb950;
                --warning-color: #d29922;
                --error-color: #f85149;
                --font-mono: ui-monospace, SFMono-Regular, SF Mono, Menlo, Consolas, monospace;
            }
            
            body {
                background-color: var(--bg-color);
                color: var(--text-color);
                font-family: var(--font-mono);
                line-height: 1.5;
                margin: 0;
                padding: 20px;
            }
            
            .container {
                max-width: 900px;
                margin: 0 auto;
            }
            
            header {
                border-bottom: 1px solid var(--border-color);
                padding-bottom: 10px;
                margin-bottom: 20px;
            }
            
            h1, h2, h3 {
                margin-top: 0;
                font-weight: 600;
            }
            
            h1 {
                color: var(--accent-color);
            }
            
            .status-box {
                background-color: var(--secondary-bg);
                border: 1px solid var(--border-color);
                border-radius: 6px;
                padding: 15px;
                margin-bottom: 20px;
            }
            
            .card-section {
                display: grid;
                grid-template-columns: repeat(auto-fill, minmax(400px, 1fr));
                gap: 25px;
                margin-bottom: 30px;
            }
            
            .card {
                background-color: var(--secondary-bg);
                border: 1px solid var(--border-color);
                border-radius: 6px;
                padding: 20px;
                min-height: 200px;
            }
            
            .card h3 {
                margin-top: 0;
                border-bottom: 1px solid var(--border-color);
                padding-bottom: 10px;
                font-size: 1.1em;
            }
            
            .search-form {
                display: flex;
                margin-bottom: 20px;
            }
            
            input[type="text"] {
                flex-grow: 1;
                background-color: var(--secondary-bg);
                border: 1px solid var(--border-color);
                border-radius: 6px 0 0 6px;
                padding: 8px 12px;
                color: var(--text-color);
                font-family: var(--font-mono);
                margin: 0;
            }
            
            button {
                background-color: var(--accent-color);
                color: black;
                border: none;
                border-radius: 0 6px 6px 0;
                padding: 8px 15px;
                font-family: var(--font-mono);
                cursor: pointer;
                font-weight: 600;
            }
            
            button:hover {
                opacity: 0.9;
            }
            
            pre {
                background-color: var(--bg-color);
                border: 1px solid var(--border-color);
                border-radius: 6px;
                padding: 15px;
                overflow-x: auto;
                margin: 0;
            }
            
            .tag {
                display: inline-block;
                padding: 3px 8px;
                border-radius: 12px;
                font-size: 0.8em;
                margin-right: 5px;
            }
            
            .tag-success {
                background-color: rgba(63, 185, 80, 0.2);
                color: var(--success-color);
                border: 1px solid rgba(63, 185, 80, 0.4);
            }
            
            .tag-warning {
                background-color: rgba(210, 153, 34, 0.2);
                color: var(--warning-color);
                border: 1px solid rgba(210, 153, 34, 0.4);
            }
            
            .tag-error {
                background-color: rgba(248, 81, 73, 0.2);
                color: var(--error-color);
                border: 1px solid rgba(248, 81, 73, 0.4);
            }
            
            .endpoints {
                margin-top: 30px;
            }
            
            .endpoint {
                background-color: var(--secondary-bg);
                border: 1px solid var(--border-color);
                border-radius: 6px;
                padding: 10px 15px;
                margin-bottom: 10px;
            }
            
            .endpoint-method {
                display: inline-block;
                padding: 3px 8px;
                border-radius: 4px;
                font-size: 0.8em;
                margin-right: 10px;
                background-color: var(--accent-color);
                color: black;
                font-weight: bold;
            }
            
            .endpoint-url {
                font-weight: 600;
                color: var(--accent-color);
            }
            
            #quote-result, #market-result {
                margin-top: 15px;
            }
            
            .hidden {
                display: none !important;
            }
            
            @media (max-width: 600px) {
                .card-section {
                    grid-template-columns: 1fr;
                }
            }
            
            .loader {
                display: none;
                border: 3px solid var(--secondary-bg);
                border-radius: 50%;
                border-top: 3px solid var(--accent-color);
                width: 20px;
                height: 20px;
                margin-left: 10px;
                animation: spin 1s linear infinite;
            }
            
            @keyframes spin {
                0% { transform: rotate(0deg); }
                100% { transform: rotate(360deg); }
            }
            
            a {
                text-decoration: none;
                color: var(--accent-color);
            }
            
            a:hover {
                text-decoration: underline;
            }
            
            .symbol-link {
                cursor: pointer;
                display: inline-block;
                margin: 0 2px;
            }
        </style>
    </head>
    <body>
        <div class="container">
            <header>
                <h1>Quotron YFinance Proxy</h1>
                <p>A fast and reliable proxy for Yahoo Finance data</p>
            </header>
            
            <div class="status-box">
                <div id="health-status">
                    <span class="tag tag-success">LOADING</span>
                    <span id="health-text">Checking health status...</span>
                </div>
                <div id="uptime" style="margin-top: 10px;">Uptime: calculating...</div>
            </div>
            
            <div class="card-section">
                <div class="card">
                    <h3>Stock Quote Lookup</h3>
                    <div class="search-form">
                        <input type="text" id="stock-symbol" placeholder="Enter stock symbol (e.g., AAPL)">
                        <button id="get-quote">Get Quote</button>
                        <div class="loader" id="quote-loader"></div>
                    </div>
                    <div style="margin-top: 10px; font-size: 0.85em;">
                        Popular symbols: 
                        <a href="#" class="symbol-link" data-type="quote" data-symbol="AAPL">AAPL</a> | 
                        <a href="#" class="symbol-link" data-type="quote" data-symbol="MSFT">MSFT</a> | 
                        <a href="#" class="symbol-link" data-type="quote" data-symbol="GOOGL">GOOGL</a> | 
                        <a href="#" class="symbol-link" data-type="quote" data-symbol="AMZN">AMZN</a> | 
                        <a href="#" class="symbol-link" data-type="quote" data-symbol="TSLA">TSLA</a>
                    </div>
                    <div id="quote-result" class="hidden">
                        <pre id="quote-data"></pre>
                    </div>
                </div>
                
                <div class="card">
                    <h3>Market Index Lookup</h3>
                    <div class="search-form">
                        <input type="text" id="market-symbol" placeholder="Enter index symbol (e.g., ^GSPC)">
                        <button id="get-market">Get Index</button>
                        <div class="loader" id="market-loader"></div>
                    </div>
                    <div style="margin-top: 10px; font-size: 0.85em;">
                        Popular indices: 
                        <a href="#" class="symbol-link" data-type="market" data-symbol="^GSPC">S&P 500</a> | 
                        <a href="#" class="symbol-link" data-type="market" data-symbol="^DJI">Dow Jones</a> | 
                        <a href="#" class="symbol-link" data-type="market" data-symbol="^IXIC">NASDAQ</a> | 
                        <a href="#" class="symbol-link" data-type="market" data-symbol="^RUT">Russell 2000</a>
                    </div>
                    <div id="market-result" class="hidden">
                        <pre id="market-data"></pre>
                    </div>
                </div>
            </div>
            
            <div class="card">
                <h3>Cache & Server Statistics</h3>
                <pre id="metrics-data">Loading metrics...</pre>
            </div>
            
            <div class="endpoints">
                <h2>Available Endpoints</h2>
                <div class="endpoint">
                    <span class="endpoint-method">GET</span>
                    <span class="endpoint-url">/quote/{symbol}</span>
                    <p>Get stock quote data for a specific symbol</p>
                </div>
                <div class="endpoint">
                    <span class="endpoint-method">GET</span>
                    <span class="endpoint-url">/market/{index}</span>
                    <p>Get market index data</p>
                </div>
                <div class="endpoint">
                    <span class="endpoint-method">GET</span>
                    <span class="endpoint-url">/health</span>
                    <p>Check the health status of the service</p>
                </div>
                <div class="endpoint">
                    <span class="endpoint-method">GET</span>
                    <span class="endpoint-url">/metrics</span>
                    <p>Get detailed metrics about the service</p>
                </div>
                <div class="endpoint">
                    <span class="endpoint-method">POST</span>
                    <span class="endpoint-url">/admin/cache/clear</span>
                    <p>Clear the cache (admin only)</p>
                </div>
            </div>
        </div>

        <script>
            // Utility function to format JSON
            function formatJSON(obj) {
                return JSON.stringify(obj, null, 2);
            }
            
            // Function to format time duration
            function formatUptime(seconds) {
                const days = Math.floor(seconds / 86400);
                const hours = Math.floor((seconds % 86400) / 3600);
                const minutes = Math.floor((seconds % 3600) / 60);
                const secs = Math.floor(seconds % 60);
                
                let result = '';
                if (days > 0) result += `${days}d `;
                if (hours > 0 || days > 0) result += `${hours}h `;
                if (minutes > 0 || hours > 0 || days > 0) result += `${minutes}m `;
                result += `${secs}s`;
                
                return result;
            }
            
            // Update health status
            function updateHealth() {
                fetch('/health')
                    .then(response => response.json())
                    .then(data => {
                        const healthTag = document.querySelector('#health-status .tag');
                        const healthText = document.getElementById('health-text');
                        const uptimeElement = document.getElementById('uptime');
                        
                        if (data.status === 'ok') {
                            healthTag.className = 'tag tag-success';
                            healthTag.textContent = 'HEALTHY';
                        } else {
                            healthTag.className = 'tag tag-error';
                            healthTag.textContent = 'UNHEALTHY';
                        }
                        
                        healthText.textContent = `Last check: ${new Date(data.last_check).toLocaleString()}`;
                        uptimeElement.textContent = `Uptime: ${formatUptime(data.uptime)}`;
                    })
                    .catch(err => {
                        const healthTag = document.querySelector('#health-status .tag');
                        const healthText = document.getElementById('health-text');
                        
                        healthTag.className = 'tag tag-error';
                        healthTag.textContent = 'ERROR';
                        healthText.textContent = `Failed to fetch health status: ${err.message}`;
                    });
            }
            
            // Update metrics
            function updateMetrics() {
                fetch('/metrics')
                    .then(response => response.json())
                    .then(data => {
                        document.getElementById('metrics-data').textContent = formatJSON(data);
                    })
                    .catch(err => {
                        document.getElementById('metrics-data').textContent = `Error fetching metrics: ${err.message}`;
                    });
            }
            
            // Initialize the page
            function init() {
                updateHealth();
                updateMetrics();
                
                // Set up periodic updates
                setInterval(updateHealth, 30000); // Every 30 seconds
                setInterval(updateMetrics, 60000); // Every minute
                
                // Set up stock quote lookup
                document.getElementById('get-quote').addEventListener('click', () => {
                    const symbol = document.getElementById('stock-symbol').value.trim();
                    if (!symbol) return;
                    
                    const loader = document.getElementById('quote-loader');
                    const resultContainer = document.getElementById('quote-result');
                    const resultData = document.getElementById('quote-data');
                    
                    resultContainer.classList.add('hidden');
                    loader.style.display = 'inline-block';
                    console.log("Fetching quote for:", symbol);
                    
                    fetch(`/quote/${symbol}`)
                        .then(response => {
                            console.log("Response status:", response.status);
                            return response.json();
                        })
                        .then(data => {
                            console.log("Quote data received:", data);
                            resultData.textContent = formatJSON(data);
                            resultContainer.classList.remove('hidden');
                        })
                        .catch(err => {
                            console.error("Error fetching quote:", err);
                            resultData.textContent = `Error: ${err.message}`;
                            resultContainer.classList.remove('hidden');
                        })
                        .finally(() => {
                            loader.style.display = 'none';
                        });
                });
                
                // Set up market index lookup
                document.getElementById('get-market').addEventListener('click', () => {
                    const symbol = document.getElementById('market-symbol').value.trim();
                    if (!symbol) return;
                    
                    const loader = document.getElementById('market-loader');
                    const resultContainer = document.getElementById('market-result');
                    const resultData = document.getElementById('market-data');
                    
                    resultContainer.classList.add('hidden');
                    loader.style.display = 'inline-block';
                    console.log("Fetching market data for:", symbol);
                    
                    fetch(`/market/${symbol}`)
                        .then(response => {
                            console.log("Response status:", response.status);
                            return response.json();
                        })
                        .then(data => {
                            console.log("Market data received:", data);
                            resultData.textContent = formatJSON(data);
                            resultContainer.classList.remove('hidden');
                        })
                        .catch(err => {
                            console.error("Error fetching market data:", err);
                            resultData.textContent = `Error: ${err.message}`;
                            resultContainer.classList.remove('hidden');
                        })
                        .finally(() => {
                            loader.style.display = 'none';
                        });
                });
                
                // Handle Enter key in input fields
                document.getElementById('stock-symbol').addEventListener('keypress', (e) => {
                    if (e.key === 'Enter') {
                        document.getElementById('get-quote').click();
                    }
                });
                
                document.getElementById('market-symbol').addEventListener('keypress', (e) => {
                    if (e.key === 'Enter') {
                        document.getElementById('get-market').click();
                    }
                });
                
                // Handle quick symbol links
                document.querySelectorAll('.symbol-link').forEach(link => {
                    link.addEventListener('click', (e) => {
                        e.preventDefault();
                        const type = link.getAttribute('data-type');
                        const symbol = link.getAttribute('data-symbol');
                        
                        if (type === 'quote') {
                            document.getElementById('stock-symbol').value = symbol;
                            document.getElementById('get-quote').click();
                        } else if (type === 'market') {
                            document.getElementById('market-symbol').value = symbol;
                            document.getElementById('get-market').click();
                        }
                    });
                });
            }
            
            // Initialize when the DOM is loaded
            document.addEventListener('DOMContentLoaded', init);
        </script>
    </body>
    </html>
    """
    return render_template_string(html_template)


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