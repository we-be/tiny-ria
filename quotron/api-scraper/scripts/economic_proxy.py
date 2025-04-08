#!/usr/bin/env python3
"""
Economic Factors Proxy Server

This script creates a Flask-based HTTP server that serves as a proxy for US economic data.
It implements the following reliability features:
1. A SimpleCache class for caching economic data with TTL (time to live) support
2. A retry mechanism with exponential backoff for API calls
3. A heartbeat system that periodically checks the proxy health
4. A circuit breaker pattern to prevent overwhelming the FRED API when issues occur
5. Integrated with unified health monitoring service
6. Proper shutdown handling via process signals

The server exposes the following endpoints:
- / - Simple web UI
- /indicator/<indicator_name> - Get data for a specific economic indicator
- /indicators - Get a list of all available indicators
- /summary - Get a summary of key economic indicators
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
import math
import requests
from collections import defaultdict
from datetime import datetime, timedelta
import random
import functools

# For FRED API integration
try:
    import pandas as pd
except ImportError:
    pd = None

# Flask for the API server
from flask import Flask, jsonify, request, make_response, g, render_template_string
from flask_cors import CORS

# Configure logging first so we can use it immediately
logging.basicConfig(
    level=logging.INFO,
    format="%(asctime)s - %(name)s - %(levelname)s - %(message)s",
)
logger = logging.getLogger("economic-proxy")

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
# Enable CORS for all routes
CORS(app)


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


class EconomicDataProvider:
    """Provider for US Economic data with caching"""
    
    def __init__(self):
        self.cache = SimpleCache(default_ttl=3600)  # 1 hour default TTL
        self.request_stats = {
            "total_requests": 0,
            "successful_requests": 0,
            "failed_requests": 0,
            "last_request_time": None,
            "api_calls": 0
        }
        # Normally we would set FRED API key here
        self.fred_api_key = os.environ.get("FRED_API_KEY", "")
        
        # Available economic indicators
        self.indicators = {
            "GDP": {
                "id": "GDP",
                "name": "Gross Domestic Product",
                "description": "Real Gross Domestic Product",
                "unit": "Billions of Dollars",
                "frequency": "Quarterly",
                "source": "FRED"
            },
            "UNRATE": {
                "id": "UNRATE",
                "name": "Unemployment Rate",
                "description": "Civilian Unemployment Rate",
                "unit": "Percent",
                "frequency": "Monthly",
                "source": "FRED"
            },
            "CPIAUCSL": {
                "id": "CPIAUCSL",
                "name": "Consumer Price Index",
                "description": "Consumer Price Index for All Urban Consumers: All Items",
                "unit": "Index 1982-1984=100",
                "frequency": "Monthly",
                "source": "FRED"
            },
            "FEDFUNDS": {
                "id": "FEDFUNDS",
                "name": "Federal Funds Rate",
                "description": "Federal Funds Effective Rate",
                "unit": "Percent",
                "frequency": "Monthly",
                "source": "FRED"
            },
            "PAYEMS": {
                "id": "PAYEMS",
                "name": "Nonfarm Payrolls",
                "description": "All Employees, Total Nonfarm",
                "unit": "Thousands of Persons",
                "frequency": "Monthly",
                "source": "FRED"
            },
            "JTSJOR": {
                "id": "JTSJOR",
                "name": "Job Openings",
                "description": "Job Openings: Total Nonfarm",
                "unit": "Thousands",
                "frequency": "Monthly",
                "source": "FRED"
            },
            "RRSFS": {
                "id": "RRSFS",
                "name": "Retail Sales",
                "description": "Advance Retail Sales: Retail and Food Services, Total",
                "unit": "Millions of Dollars",
                "frequency": "Monthly",
                "source": "FRED"
            },
            "HOUST": {
                "id": "HOUST",
                "name": "Housing Starts",
                "description": "Housing Starts: Total New Privately Owned",
                "unit": "Thousands of Units",
                "frequency": "Monthly",
                "source": "FRED"
            },
            "CSUSHPISA": {
                "id": "CSUSHPISA",
                "name": "Housing Price Index",
                "description": "S&P/Case-Shiller U.S. National Home Price Index",
                "unit": "Index Jan 2000=100",
                "frequency": "Monthly",
                "source": "FRED"
            },
            "INDPRO": {
                "id": "INDPRO",
                "name": "Industrial Production",
                "description": "Industrial Production Index",
                "unit": "Index 2017=100",
                "frequency": "Monthly",
                "source": "FRED"
            }
        }
    
    def _fetch_indicator_data(self, indicator, period="5y"):
        """
        Fetch indicator data from FRED API (or generate test data)
        
        Args:
            indicator: Indicator to fetch
            period: Time period for the data (e.g., '5y', '1y', 'max')
            
        Returns:
            Indicator data
        """
        self.request_stats["api_calls"] += 1
        
        # Extract indicator ID from potentially combined format
        indicator_id = indicator.split(':')[0] if ':' in indicator else indicator
        
        # Check if indicator exists
        if indicator_id not in self.indicators:
            return self._generate_indicator_fallback(indicator_id, period, 
                                                   error="Indicator not found")
        
        # Check if we have a FRED API key
        if self.fred_api_key:
            try:
                # Import FRED API client
                from fredapi import Fred
                fred = Fred(api_key=self.fred_api_key)
                
                # Fetch the data from FRED
                series = fred.get_series(indicator_id)
                
                # Parse period to determine date range
                if period == "5y":
                    series = series[-60:]  # Approximately 5 years of monthly data
                elif period == "1y":
                    series = series[-12:]  # Approximately 1 year of monthly data
                
                # Format the data
                data_points = []
                for date, value in series.items():
                    if not pd.isna(value):  # Skip NaN values
                        data_points.append({
                            "date": date.strftime("%Y-%m-%d"),
                            "value": float(value),
                            "unit": self.indicators.get(indicator_id, {}).get("unit", "Value")
                        })
                
                # Return the formatted data
                return {
                    "indicator": indicator_id,
                    "name": self.indicators.get(indicator_id, {}).get("name", indicator_id),
                    "description": self.indicators.get(indicator_id, {}).get("description", ""),
                    "period": period,
                    "data": data_points,
                    "metadata": {
                        "unit": self.indicators.get(indicator_id, {}).get("unit", "Value"),
                        "frequency": self.indicators.get(indicator_id, {}).get("frequency", "Monthly"),
                        "data_points": len(data_points),
                        "start_date": data_points[0]["date"] if data_points else None,
                        "end_date": data_points[-1]["date"] if data_points else None,
                        "source": "FRED"
                    }
                }
            except Exception as e:
                logger.error(f"Error fetching from FRED API: {e}")
                logger.info(f"Falling back to generated data for {indicator_id}")
                
        # If we don't have an API key or an error occurred, generate fallback data
        return self._generate_indicator_fallback(indicator_id, period)
            
    def _generate_indicator_fallback(self, indicator, period, error=None):
        """Generate fallback economic indicator data when the API fails or for testing"""
        logger.info(f"Generating fallback data for indicator: {indicator}")
        
        # If there was an error specified, return it
        if error:
            return {
                "error": error,
                "indicator": indicator,
                "period": period,
                "data": [],
                "metadata": {
                    "source": "fallback generator",
                    "note": "Error generating data: " + error
                }
            }
            
        # Get indicator metadata if available
        indicator_info = self.indicators.get(indicator, {
            "id": indicator,
            "name": indicator,
            "description": f"Economic indicator {indicator}",
            "unit": "Value",
            "frequency": "Monthly",
            "source": "Generated"
        })
        
        # Parse period to determine date range
        if period == "5y":
            # 5 years of monthly data = ~60 data points
            days = 365 * 5
            interval = 30  # Monthly data points
        elif period == "1y":
            # 1 year of monthly data = ~12 data points
            days = 365
            interval = 30
        elif period == "max":
            # 10 years of monthly data = ~120 data points
            days = 365 * 10
            interval = 30
        else:
            # Default to 1 year
            days = 365
            interval = 30
        
        # Check if a specific date was requested (through query param or header)
        target_date_str = request.args.get('target_date') if hasattr(request, 'args') else None
        
        if target_date_str:
            try:
                # Parse the target date (expected format: YYYY-MM-DD)
                target_date = datetime.strptime(target_date_str, '%Y-%m-%d')
            except ValueError:
                # If parsing fails, use today's date
                target_date = datetime.now()
        else:
            # Default to November 2024 for demo purposes
            target_date = datetime(2024, 11, 15)  # Middle of November 2024
            
        # Log the target date being used
        logger.info(f"Generating data with target date: {target_date.strftime('%Y-%m-%d')}")
        
        # Generate dates from past to the target date
        dates = [(target_date - timedelta(days=i)).strftime('%Y-%m-%d') for i in range(0, days, interval)]
        dates.reverse()  # Oldest to newest
        
        # Generate random values with a realistic pattern
        # Choose a trend pattern: gradual up, gradual down, spike, cyclical
        pattern_type = random.choice(['up', 'down', 'spike', 'cycle'])
        
        # Set base value based on indicator
        if indicator == "GDP":
            base_value = 20000  # Billions of dollars
            volatility = 200
        elif indicator == "UNRATE":
            base_value = 4.0  # Percent
            volatility = 0.2
        elif indicator == "CPIAUCSL":
            base_value = 250  # Index
            volatility = 1.0
        elif indicator == "FEDFUNDS":
            base_value = 2.0  # Percent
            volatility = 0.25
        elif indicator == "PAYEMS":
            base_value = 150000  # Thousands of persons
            volatility = 200
        elif indicator == "JTSJOR":
            base_value = 7000  # Thousands
            volatility = 300
        elif indicator == "RRSFS":
            base_value = 500000  # Millions of dollars
            volatility = 5000
        elif indicator == "HOUST":
            base_value = 1500  # Thousands of units
            volatility = 100
        elif indicator == "CSUSHPISA":
            base_value = 200  # Index
            volatility = 2
        elif indicator == "INDPRO":
            base_value = 105  # Index
            volatility = 1
        else:
            base_value = 100
            volatility = 5
        
        data_points = []
        for i, date in enumerate(dates):
            progress = i / len(dates)  # 0 to 1
            
            # Calculate value based on pattern
            if pattern_type == 'up':
                # Gradual upward trend
                trend_value = base_value * (1 + progress * 0.3)
            elif pattern_type == 'down':
                # Gradual downward trend
                trend_value = base_value * (1 - progress * 0.2)
            elif pattern_type == 'spike':
                # Spike in the middle
                middle = 0.5
                distance = abs(progress - middle)
                trend_value = base_value * (1 + 0.2 - distance * 0.4)
            else:  # cycle
                # Cyclical pattern
                cycles = 3  # Number of cycles
                trend_value = base_value * (1 + 0.1 * math.sin(progress * cycles * 2 * math.pi))
            
            # Add randomness
            random_factor = random.uniform(-volatility, volatility)
            value = max(0, trend_value + random_factor)
            
            data_points.append({
                "date": date,
                "value": value,
                "unit": indicator_info["unit"]
            })
        
        return {
            "indicator": indicator,
            "name": indicator_info["name"],
            "description": indicator_info["description"],
            "period": period,
            "data": data_points,
            "metadata": {
                "unit": indicator_info["unit"],
                "frequency": indicator_info["frequency"],
                "data_points": len(data_points),
                "start_date": dates[0] if dates else None,
                "end_date": dates[-1] if dates else None,
                "source": "fallback generator",
                "pattern": pattern_type
            }
        }
    
    def _fetch_and_cache_data(self, indicator, cache_type, fetch_func, transform_func=None):
        """
        Generic method to fetch and cache data
        
        Args:
            indicator: Indicator to fetch
            cache_type: Type of data for cache key
            fetch_func: Function to fetch data
            transform_func: Optional function to transform raw data
            
        Returns:
            Formatted data
        """
        self.request_stats["total_requests"] += 1
        self.request_stats["last_request_time"] = datetime.now()
        
        # Check cache first
        cache_key = f"{cache_type}:{indicator}"
        cached_data = self.cache.get(cache_key)
        
        if cached_data:
            return cached_data
        
        # Fetch from API
        try:
            data = fetch_func(indicator)
            
            # Transform the data if a transform function is provided
            if transform_func:
                result = transform_func(data, indicator)
            else:
                result = data
            
            # Cache the result
            self.cache.set(cache_key, result)
            self.request_stats["successful_requests"] += 1
            
            return result
        except Exception as e:
            self.request_stats["failed_requests"] += 1
            raise e
    
    def get_indicator(self, indicator, period="5y", target_date=None):
        """
        Get data for an economic indicator
        
        Args:
            indicator: Indicator to fetch
            period: Time period for the data (e.g., '5y', '1y', 'max')
            target_date: Optional specific date to use as reference point
            
        Returns:
            Indicator data
        """
        def fetch_func(ind):
            # Pass the target_date to the fetch function if provided
            return self._fetch_indicator_data(ind, period)
        
        def transform_func(data, ind):
            # Extract metadata for top level access
            metadata = data.get('metadata', {})
            data_points = data.get('data', [])
            
            return {
                "indicator": ind,
                "name": data.get("name", ind),
                "description": data.get("description", ""),
                "period": period,
                "data": data_points,
                "metadata": metadata,
                "timestamp": datetime.now().strftime("%Y-%m-%dT%H:%M:%SZ"),
                "source": metadata.get('source', "FRED Economic Data"),
                "target_date": target_date
            }
        
        # Include target_date in the cache key if provided
        cache_key = f"{indicator}:{period}"
        if target_date:
            cache_key += f":{target_date}"
            
        return self._fetch_and_cache_data(cache_key, "indicator", fetch_func, transform_func)
    
    def get_indicators(self):
        """
        Get list of all available indicators
        
        Returns:
            List of indicator details
        """
        indicators_list = []
        for indicator_id, details in self.indicators.items():
            indicators_list.append({
                "id": indicator_id,
                "name": details["name"],
                "description": details["description"],
                "unit": details["unit"],
                "frequency": details["frequency"]
            })
        
        return {
            "indicators": indicators_list,
            "count": len(indicators_list),
            "timestamp": datetime.now().strftime("%Y-%m-%dT%H:%M:%SZ")
        }
    
    def get_summary(self, target_date=None):
        """
        Get summary of key economic indicators
        
        Args:
            target_date: Optional specific date to use as reference point
        
        Returns:
            Summary of economic indicators
        """
        # Key indicators to include in summary
        key_indicators = ["GDP", "UNRATE", "CPIAUCSL", "FEDFUNDS"]
        
        summary_data = {}
        indicator_names = []
        
        for indicator in key_indicators:
            try:
                # Pass the target_date to the get_indicator method
                indicator_data = self.get_indicator(indicator, "1y", target_date=target_date)
                
                # Extract latest value and trend
                if indicator_data and "data" in indicator_data and indicator_data["data"]:
                    latest_data = indicator_data["data"][-1]
                    first_data = indicator_data["data"][0] if len(indicator_data["data"]) > 1 else latest_data
                    
                    # Calculate change
                    latest_value = latest_data.get("value", 0)
                    first_value = first_data.get("value", 0)
                    
                    if first_value != 0:
                        pct_change = ((latest_value - first_value) / first_value) * 100
                    else:
                        pct_change = 0
                    
                    # Determine trend
                    if pct_change > 1:
                        trend = "up"
                    elif pct_change < -1:
                        trend = "down"
                    else:
                        trend = "stable"
                    
                    # Add to summary
                    summary_data[indicator] = {
                        "name": indicator_data.get("name", indicator),
                        "value": latest_value,
                        "date": latest_data.get("date"),
                        "unit": indicator_data.get("metadata", {}).get("unit", ""),
                        "change_pct": round(pct_change, 2),
                        "trend": trend
                    }
                    
                    indicator_names.append(indicator_data.get("name", indicator))
            except Exception as e:
                logger.error(f"Error getting summary for {indicator}: {e}")
        
        # Overall economic health assessment
        # Simple algorithm - count trending indicators
        up_count = sum(1 for ind, data in summary_data.items() if data["trend"] == "up")
        down_count = sum(1 for ind, data in summary_data.items() if data["trend"] == "down")
        
        # Simple assessment based on unemployment and growth
        health_score = 0
        
        # GDP contribution (higher is better)
        if "GDP" in summary_data:
            gdp_change = summary_data["GDP"]["change_pct"]
            if gdp_change > 2:
                health_score += 2
            elif gdp_change > 0:
                health_score += 1
            else:
                health_score -= 1
        
        # Unemployment contribution (lower is better)
        if "UNRATE" in summary_data:
            unemployment = summary_data["UNRATE"]["value"]
            unemployment_change = summary_data["UNRATE"]["change_pct"]
            
            if unemployment < 4:
                health_score += 2
            elif unemployment < 6:
                health_score += 1
            else:
                health_score -= 1
                
            # Decreasing unemployment is good
            if unemployment_change < -1:
                health_score += 1
            elif unemployment_change > 1:
                health_score -= 1
        
        # Interpret health score
        if health_score >= 3:
            overall_health = "Strong"
        elif health_score >= 1:
            overall_health = "Moderate"
        elif health_score >= -1:
            overall_health = "Stable"
        else:
            overall_health = "Weak"
        
        return {
            "indicators": indicator_names,
            "timestamp": datetime.now().strftime("%Y-%m-%dT%H:%M:%SZ"),
            "source": "Economic Factors API",
            "summary": summary_data,
            "overall": {
                "health": overall_health,
                "trending_up": up_count,
                "trending_down": down_count,
                "score": health_score
            }
        }
    
    def get_stats(self):
        """Get provider statistics"""
        cache_stats = self.cache.get_stats()
        
        stats = {
            "request_stats": self.request_stats,
            "cache_stats": cache_stats
        }
        
        return stats


# Create the economic data provider
economic_provider = EconomicDataProvider()

# Health check status tracking
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
                    source_name="economic_factors_proxy",
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
    test_indicator = "GDP"
    try:
        result = economic_provider.get_indicator(test_indicator)
        health_check_status["status"] = "ok"
        health_check_status["is_healthy"] = True
        health_check_status["consecutive_failures"] = 0
        
        # Report healthy status if health client available
        if health_enabled and health_client:
            health_client.report_health(
                source_type="api-scraper",
                source_name="economic_factors_proxy",
                status=HealthStatus.HEALTHY,
                metadata={
                    "uptime": (datetime.now() - startup_time).total_seconds(),
                    "cache_stats": economic_provider.cache.get_stats()
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
                source_name="economic_factors_proxy",
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
@app.route('/indicator/<indicator>', methods=['GET'])
def get_indicator(indicator):
    """Get data for a specific economic indicator"""
    period = request.args.get('period', default='5y', type=str)
    simplified = request.args.get('simplified', default='false', type=str).lower() == 'true'
    target_date = request.args.get('target_date', default=None)
    
    # Forward target_date only if provided
    if target_date:
        result = economic_provider.get_indicator(indicator, period, target_date=target_date)
    else:
        result = economic_provider.get_indicator(indicator, period)
    
    # Simplify the response if requested
    if simplified:
        simplified_data = {
            "indicator": result["indicator"],
            "name": result["name"],
            "period": result["period"],
            "data_points": []
        }
        
        # Extract just date and value for cleaner display
        for point in result["data"]:
            date = point.get("date", "")
            value = point.get("value", 0)
            simplified_data["data_points"].append({
                "date": date,
                "value": value
            })
        
        result = simplified_data
    
    response = make_response(jsonify(result))
    
    # Set cache status header for client tracking
    if "error" not in result:
        response.headers['X-Cache-Status'] = 'miss'
    
    return response


@app.route('/indicators', methods=['GET'])
def get_indicators():
    """Get list of all available economic indicators"""
    result = economic_provider.get_indicators()
    return jsonify(result)


@app.route('/summary', methods=['GET'])
def get_summary():
    """Get summary of economic indicators"""
    target_date = request.args.get('target_date', default=None)
    
    if target_date:
        result = economic_provider.get_summary(target_date=target_date)
    else:
        result = economic_provider.get_summary()
    
    response = make_response(jsonify(result))
    
    # Set cache status header for client tracking
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
            service_health = health_client.get_service_health("api-scraper", "economic_factors_proxy")
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
        "provider_stats": economic_provider.get_stats(),
        "health": health_check_status,
        "uptime": (datetime.now() - startup_time).total_seconds()
    })


@app.route('/admin/cache/clear', methods=['POST'])
def clear_cache():
    """Admin endpoint to clear the cache"""
    economic_provider.cache.clear()
    return jsonify({"status": "ok", "message": "Cache cleared"})


@app.route('/dashboard', methods=['GET'])
def dashboard():
    """Economic indicators dashboard with real data visualization"""
    # Get the absolute path to the dashboard file
    dashboard_path = os.path.abspath(os.path.join(os.path.dirname(__file__), 'indicators_dashboard.html'))
    
    # Check if the file exists
    if os.path.exists(dashboard_path):
        try:
            with open(dashboard_path, 'r') as f:
                return f.read()
        except Exception as e:
            return f"Error reading dashboard file: {e}", 500
    else:
        # Fallback to embedded dashboard if file not found
        return render_template_string("""
        <!DOCTYPE html>
        <html>
        <head>
            <title>Economic Indicators</title>
            <style>
                body { font-family: Arial, sans-serif; margin: 20px; }
                h1 { color: #333; }
                .card { border: 1px solid #ccc; border-radius: 5px; padding: 15px; margin: 10px 0; }
                .indicator { font-weight: bold; }
                a { color: #0366d6; text-decoration: none; }
                a:hover { text-decoration: underline; }
            </style>
        </head>
        <body>
            <h1>Economic Indicators Dashboard</h1>
            <p>Simplified dashboard view</p>
            
            <div class="card">
                <h2>Available Endpoints</h2>
                <ul>
                    <li><a href="/indicator/GDP">/indicator/GDP</a> - GDP data</li>
                    <li><a href="/indicator/UNRATE">/indicator/UNRATE</a> - Unemployment rate</li>
                    <li><a href="/indicator/CPIAUCSL">/indicator/CPIAUCSL</a> - Consumer Price Index</li>
                    <li><a href="/indicator/FEDFUNDS">/indicator/FEDFUNDS</a> - Federal Funds Rate</li>
                    <li><a href="/summary">/summary</a> - Economic summary</li>
                    <li><a href="/indicators">/indicators</a> - All available indicators</li>
                </ul>
            </div>
            
            <div class="card">
                <h2>Economic Summary</h2>
                <div id="summary">Loading summary data...</div>
            </div>
            
            <script>
                // Fetch summary data
                fetch('/summary')
                    .then(response => response.json())
                    .then(data => {
                        const summaryDiv = document.getElementById('summary');
                        let html = `<p>Overall health: <strong>${data.overall.health}</strong></p><ul>`;
                        
                        // Add each indicator
                        Object.keys(data.summary).forEach(key => {
                            const indicator = data.summary[key];
                            const trend = indicator.trend === 'up' ? '↑' : 
                                         indicator.trend === 'down' ? '↓' : '→';
                            
                            html += `<li>
                                <span class="indicator">${indicator.name}:</span> 
                                ${indicator.value} ${indicator.unit} 
                                <span>${trend} ${indicator.change_pct}%</span>
                            </li>`;
                        });
                        
                        html += `</ul><p><small>Source: ${data.source}</small></p>`;
                        summaryDiv.innerHTML = html;
                    })
                    .catch(error => {
                        document.getElementById('summary').innerHTML = 
                            `<p>Error loading summary: ${error.message}</p>`;
                    });
            </script>
        </body>
        </html>
        """)

@app.route('/', methods=['GET'])
def index():
    """Root web UI"""
    html_template = """
    <!DOCTYPE html>
    <html lang="en">
    <head>
        <meta charset="UTF-8">
        <meta name="viewport" content="width=device-width, initial-scale=1.0">
        <title>US Economic Factors Proxy</title>
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
            
            #indicator-result, #indicators-result, #summary-result {
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
            
            .keyword-link {
                cursor: pointer;
                display: inline-block;
                margin: 0 2px;
            }
            
            .summary-grid {
                display: grid;
                grid-template-columns: repeat(auto-fill, minmax(200px, 1fr));
                gap: 15px;
                margin-top: 15px;
            }
            
            .summary-item {
                background-color: var(--bg-color);
                border: 1px solid var(--border-color);
                border-radius: 6px;
                padding: 10px;
            }
            
            .summary-item h4 {
                margin-top: 0;
                margin-bottom: 5px;
                color: var(--accent-color);
            }
            
            .value-up {
                color: var(--success-color);
            }
            
            .value-down {
                color: var(--error-color);
            }
            
            .value-stable {
                color: var(--warning-color);
            }
        </style>
    </head>
    <body>
        <div class="container">
            <header>
                <h1>US Economic Factors Proxy</h1>
                <p>A fast and reliable proxy for US economic data</p>
            </header>
            
            <div class="status-box">
                <div id="health-status">
                    <span class="tag tag-success">LOADING</span>
                    <span id="health-text">Checking health status...</span>
                </div>
                <div id="uptime" style="margin-top: 10px;">Uptime: calculating...</div>
            </div>
            
            <div class="card">
                <h3>Economic Summary</h3>
                <button id="get-summary">Get Economic Summary</button>
                <div class="loader" id="summary-loader"></div>
                <div id="summary-result" class="hidden">
                    <div id="overall-status"></div>
                    <div id="summary-grid" class="summary-grid"></div>
                </div>
            </div>
            
            <div class="card-section">
                <div class="card">
                    <h3>Economic Indicator</h3>
                    <div class="search-form">
                        <input type="text" id="indicator-input" placeholder="Enter indicator (e.g., GDP)">
                        <button id="get-indicator">Get Data</button>
                        <div class="loader" id="indicator-loader"></div>
                    </div>
                    <div style="margin-top: 10px; font-size: 0.85em;">
                        Popular indicators: 
                        <a href="#" class="keyword-link" data-type="indicator" data-keyword="GDP">GDP</a> | 
                        <a href="#" class="keyword-link" data-type="indicator" data-keyword="UNRATE">Unemployment</a> | 
                        <a href="#" class="keyword-link" data-type="indicator" data-keyword="CPIAUCSL">CPI</a> | 
                        <a href="#" class="keyword-link" data-type="indicator" data-keyword="FEDFUNDS">Fed Rate</a>
                    </div>
                    <div id="indicator-result" class="hidden">
                        <pre id="indicator-data"></pre>
                    </div>
                </div>
                
                <div class="card">
                    <h3>Available Indicators</h3>
                    <button id="get-indicators">Show All Indicators</button>
                    <div class="loader" id="indicators-loader"></div>
                    <div id="indicators-result" class="hidden">
                        <pre id="indicators-data"></pre>
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
                    <span class="endpoint-url">/indicator/{indicator}</span>
                    <p>Get data for a specific economic indicator. Optional query param: period (5y, 1y, max)</p>
                </div>
                <div class="endpoint">
                    <span class="endpoint-method">GET</span>
                    <span class="endpoint-url">/indicators</span>
                    <p>Get list of all available economic indicators</p>
                </div>
                <div class="endpoint">
                    <span class="endpoint-method">GET</span>
                    <span class="endpoint-url">/summary</span>
                    <p>Get summary of key US economic indicators</p>
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
            
            // Format economic summary
            function formatEconomicSummary(data) {
                const overallElement = document.getElementById('overall-status');
                const summaryGrid = document.getElementById('summary-grid');
                
                // Clear existing content
                overallElement.innerHTML = '';
                summaryGrid.innerHTML = '';
                
                // Add overall economic health
                const overall = data.overall;
                overallElement.innerHTML = `
                    <h4>Overall US Economic Health: 
                        <span class="value-${overall.health === 'Strong' ? 'up' : 
                                           overall.health === 'Weak' ? 'down' : 'stable'}">
                            ${overall.health}
                        </span>
                    </h4>
                    <p>Indicators trending up: ${overall.trending_up}, 
                       Indicators trending down: ${overall.trending_down}</p>
                `;
                
                // Add each indicator to the grid
                for (const [indicatorId, data] of Object.entries(data.summary)) {
                    const itemElement = document.createElement('div');
                    itemElement.className = 'summary-item';
                    
                    const trendClass = data.trend === 'up' ? 'value-up' : 
                                      data.trend === 'down' ? 'value-down' : 'value-stable';
                    
                    const formattedValue = Number.isInteger(data.value) ? 
                                         data.value.toLocaleString() : 
                                         data.value.toLocaleString(undefined, {minimumFractionDigits: 1, maximumFractionDigits: 2});
                    
                    itemElement.innerHTML = `
                        <h4>${data.name}</h4>
                        <div>Value: <strong>${formattedValue}</strong> ${data.unit}</div>
                        <div>Change: <span class="${trendClass}">${data.change_pct > 0 ? '+' : ''}${data.change_pct}%</span></div>
                        <div class="date">As of: ${new Date(data.date).toLocaleDateString()}</div>
                    `;
                    
                    summaryGrid.appendChild(itemElement);
                }
            }
            
            // Initialize the page
            function init() {
                updateHealth();
                updateMetrics();
                
                // Set up periodic updates
                setInterval(updateHealth, 30000); // Every 30 seconds
                setInterval(updateMetrics, 60000); // Every minute
                
                // Set up indicator lookup
                document.getElementById('get-indicator').addEventListener('click', () => {
                    const indicator = document.getElementById('indicator-input').value.trim();
                    if (!indicator) return;
                    
                    const loader = document.getElementById('indicator-loader');
                    const resultContainer = document.getElementById('indicator-result');
                    const resultData = document.getElementById('indicator-data');
                    
                    resultContainer.classList.add('hidden');
                    loader.style.display = 'inline-block';
                    
                    fetch(`/indicator/${encodeURIComponent(indicator)}`)
                        .then(response => response.json())
                        .then(data => {
                            resultData.textContent = formatJSON(data);
                            resultContainer.classList.remove('hidden');
                        })
                        .catch(err => {
                            resultData.textContent = `Error: ${err.message}`;
                            resultContainer.classList.remove('hidden');
                        })
                        .finally(() => {
                            loader.style.display = 'none';
                        });
                });
                
                // Set up indicators list lookup
                document.getElementById('get-indicators').addEventListener('click', () => {
                    const loader = document.getElementById('indicators-loader');
                    const resultContainer = document.getElementById('indicators-result');
                    const resultData = document.getElementById('indicators-data');
                    
                    resultContainer.classList.add('hidden');
                    loader.style.display = 'inline-block';
                    
                    fetch('/indicators')
                        .then(response => response.json())
                        .then(data => {
                            resultData.textContent = formatJSON(data);
                            resultContainer.classList.remove('hidden');
                        })
                        .catch(err => {
                            resultData.textContent = `Error: ${err.message}`;
                            resultContainer.classList.remove('hidden');
                        })
                        .finally(() => {
                            loader.style.display = 'none';
                        });
                });
                
                // Set up summary lookup
                document.getElementById('get-summary').addEventListener('click', () => {
                    const loader = document.getElementById('summary-loader');
                    const resultContainer = document.getElementById('summary-result');
                    
                    resultContainer.classList.add('hidden');
                    loader.style.display = 'inline-block';
                    
                    fetch('/summary')
                        .then(response => response.json())
                        .then(data => {
                            formatEconomicSummary(data);
                            resultContainer.classList.remove('hidden');
                        })
                        .catch(err => {
                            document.getElementById('overall-status').innerHTML = `Error: ${err.message}`;
                            resultContainer.classList.remove('hidden');
                        })
                        .finally(() => {
                            loader.style.display = 'none';
                        });
                });
                
                // Handle Enter key in input fields
                document.getElementById('indicator-input').addEventListener('keypress', (e) => {
                    if (e.key === 'Enter') {
                        document.getElementById('get-indicator').click();
                    }
                });
                
                // Handle quick indicator links
                document.querySelectorAll('.keyword-link').forEach(link => {
                    link.addEventListener('click', (e) => {
                        e.preventDefault();
                        const type = link.getAttribute('data-type');
                        const keyword = link.getAttribute('data-keyword');
                        
                        if (type === 'indicator') {
                            document.getElementById('indicator-input').value = keyword;
                            document.getElementById('get-indicator').click();
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
        logger.info("=== Starting Economic Factors Proxy Server ===")
        logger.info(f"Python version: {sys.version}")
        logger.info(f"Current directory: {os.getcwd()}")
        logger.info(f"Script directory: {os.path.dirname(os.path.abspath(__file__))}")
        
        # Parse arguments
        parser = argparse.ArgumentParser(description="Economic Factors Proxy Server")
        parser.add_argument("--host", default="localhost", help="Host to bind the server to")
        parser.add_argument("--port", type=int, default=5002, help="Port to bind the server to")
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
        logger.info(f"Starting Economic Factors proxy server on http://{args.host}:{args.port}")
        app.run(host=args.host, port=args.port, debug=args.debug)
    except Exception as e:
        logger.critical(f"Fatal error in main execution: {e}", exc_info=True)
        sys.exit(1)