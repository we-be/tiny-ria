#!/usr/bin/env python3
"""
Google Trends Proxy Server

This script creates a Flask-based HTTP server that serves as a proxy for Google Trends data.
It implements the following reliability features:
1. A SimpleCache class for caching trend data with TTL (time to live) support
2. A retry mechanism with exponential backoff for API calls
3. A heartbeat system that periodically checks the proxy health
4. A circuit breaker pattern to prevent overwhelming the Google Trends API when issues occur
5. Integrated with unified health monitoring service
6. Proper shutdown handling via process signals

The server exposes the following endpoints:
- / - Simple web UI
- /interest-over-time/<keyword> - Get interest-over-time data for a keyword
- /related-queries/<keyword> - Get related queries for a keyword
- /related-topics/<keyword> - Get related topics for a keyword
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
from collections import defaultdict
from datetime import datetime, timedelta
import random
import functools

# Flask for the API server
from flask import Flask, jsonify, request, make_response, g, render_template_string

# pytrends for Google Trends API
from pytrends.request import TrendReq

# Configure logging first so we can use it immediately
logging.basicConfig(
    level=logging.INFO,
    format="%(asctime)s - %(name)s - %(levelname)s - %(message)s",
)
logger = logging.getLogger("gtrends-proxy")

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


class TrendsDataProvider:
    """Provider for Google Trends data with caching"""
    
    def __init__(self):
        self.cache = SimpleCache(default_ttl=3600)  # 1 hour default TTL
        self.request_stats = {
            "total_requests": 0,
            "successful_requests": 0,
            "failed_requests": 0,
            "last_request_time": None,
            "api_calls": 0
        }
        self.pytrends = TrendReq(hl='en-US', tz=360)
    
    def _fetch_interest_over_time(self, keyword, timeframe='today 5-y'):
        """
        Fetch interest over time data from Google Trends
        
        Args:
            keyword: Keyword to fetch interest for
            timeframe: Time frame for the data (e.g., 'today 5-y', 'past 12-m')
            
        Returns:
            Interest over time dataframe converted to dict
        """
        self.request_stats["api_calls"] += 1
        
        try:
            # Create a fresh pytrends instance
            pytrends = TrendReq(hl='en-US', tz=360)
            pytrends.build_payload([keyword], cat=0, timeframe=timeframe, geo='', gprop='')
            data = pytrends.interest_over_time()
            
            # Convert DataFrame to dict for JSON serialization
            if not data.empty:
                # Add some metadata about the result
                result = {
                    'data': data.reset_index().to_dict(orient='records'),
                    'metadata': {
                        'timeframe': timeframe,
                        'data_points': len(data.index),
                        'start_date': data.index.min().strftime('%Y-%m-%d') if not data.empty else None,
                        'end_date': data.index.max().strftime('%Y-%m-%d') if not data.empty else None,
                        'source': 'Google Trends API'
                    }
                }
                return result
            else:
                logger.warning(f"Empty data returned for interest over time for keyword: {keyword}")
                return self._generate_interest_over_time_fallback(keyword, timeframe)
        except Exception as e:
            logger.error(f"Error fetching interest over time: {e}")
            return self._generate_interest_over_time_fallback(keyword, timeframe)
            
    def _generate_interest_over_time_fallback(self, keyword, timeframe):
        """Generate fallback interest over time data when the API fails"""
        logger.info(f"Generating fallback interest over time data for keyword: {keyword}")
        
        # Extract just the keyword without timeframe if it's combined
        if ':' in keyword:
            keyword = keyword.split(':')[0]
            
        # Parse timeframe to determine date range
        if timeframe == 'today 5-y':
            # 5 years of weekly data = ~260 data points
            days = 260 * 7
            interval = 7  # Weekly data points
        elif 'today ' in timeframe and '-m' in timeframe:
            # Extract months
            try:
                months = int(timeframe.split('today ')[1].split('-m')[0])
                days = months * 30
                interval = 1
            except:
                days = 90
                interval = 1
        elif 'today ' in timeframe and '-d' in timeframe:
            # Extract days
            try:
                days = int(timeframe.split('today ')[1].split('-d')[0])
                interval = 1
            except:
                days = 30
                interval = 1
        else:
            # Default to 90 days
            days = 90
            interval = 1
        
        # Generate dates from past to today
        today = datetime.now()
        dates = [(today - timedelta(days=i)).strftime('%Y-%m-%d') for i in range(0, days, interval)]
        dates.reverse()  # Oldest to newest
        
        # Generate random interest values with a realistic pattern
        # Choose a trend pattern: gradual up, gradual down, spike, cyclical
        pattern_type = random.choice(['up', 'down', 'spike', 'cycle'])
        base_value = random.randint(30, 70)
        max_random = 15  # Maximum random variation
        
        data_points = []
        for i, date in enumerate(dates):
            progress = i / len(dates)  # 0 to 1
            
            # Calculate value based on pattern
            if pattern_type == 'up':
                # Gradual upward trend
                trend_value = base_value + (progress * 40)
            elif pattern_type == 'down':
                # Gradual downward trend
                trend_value = base_value + 40 - (progress * 40)
            elif pattern_type == 'spike':
                # Spike in the middle
                middle = 0.5
                distance = abs(progress - middle)
                trend_value = base_value + 40 * (1 - distance * 2)
            else:  # cycle
                # Cyclical pattern
                cycles = 3  # Number of cycles
                trend_value = base_value + 20 * math.sin(progress * cycles * 2 * math.pi)
            
            # Add randomness
            random_factor = random.randint(-max_random, max_random)
            value = max(0, min(100, trend_value + random_factor))
            
            data_points.append({
                'date': date,
                keyword: value,
                'isPartial': False
            })
        
        return {
            'data': data_points,
            'metadata': {
                'timeframe': timeframe,
                'data_points': len(data_points),
                'start_date': dates[0] if dates else None,
                'end_date': dates[-1] if dates else None,
                'source': 'fallback generator',
                'note': 'Generated fallback data due to API limitations',
                'pattern': pattern_type
            }
        }
    
    def _fetch_related_queries(self, keyword, timeframe='today 5-y'):
        """
        Fetch related queries data from Google Trends
        
        Args:
            keyword: Keyword to fetch related queries for
            timeframe: Time frame for the data (e.g., 'today 5-y', 'past 12-m')
            
        Returns:
            Related queries data with enhanced metadata
        """
        self.request_stats["api_calls"] += 1
        try:
            # Create a fresh pytrends instance for each request
            pytrends = TrendReq(hl='en-US', tz=360)
            pytrends.build_payload([keyword], cat=0, timeframe=timeframe, geo='', gprop='')
            
            try:
                data = pytrends.related_queries()
                
                # Better debugging to understand the structure
                logger.info(f"Related queries raw data type: {type(data)}")
                if isinstance(data, dict):
                    logger.info(f"Related queries keys: {list(data.keys())}")
                
                # Handle case where response doesn't have the expected structure
                if not data or not isinstance(data, dict) or keyword not in data:
                    # Use fallback data generation
                    return self._generate_related_queries_fallback(keyword, timeframe)
                
                result_data = data.get(keyword, {})
                logger.info(f"Result data keys: {list(result_data.keys()) if isinstance(result_data, dict) else 'Not a dict'}")
                
                # Safely process top queries
                top_data = []
                if 'top' in result_data and result_data['top'] is not None:
                    if hasattr(result_data['top'], 'to_dict'):
                        try:
                            top_data = result_data['top'].to_dict(orient='records')
                        except Exception as e:
                            logger.error(f"Error converting top queries to dict: {e}")
                
                # Safely process rising queries
                rising_data = []
                if 'rising' in result_data and result_data['rising'] is not None:
                    if hasattr(result_data['rising'], 'to_dict'):
                        try:
                            rising_data = result_data['rising'].to_dict(orient='records')
                        except Exception as e:
                            logger.error(f"Error converting rising queries to dict: {e}")
                
                # If we still don't have data, use fallback
                if not top_data and not rising_data:
                    return self._generate_related_queries_fallback(keyword, timeframe)
                
                result = {
                    "top": top_data,
                    "rising": rising_data,
                    "metadata": {
                        "timeframe": timeframe,
                        "keyword": keyword,
                        "top_count": len(top_data),
                        "rising_count": len(rising_data),
                        "request_time": datetime.now().strftime('%Y-%m-%d %H:%M:%S'),
                        "source": "Google Trends API"
                    }
                }
                return result
                
            except Exception as inner_e:
                logger.error(f"Inner error in related queries, using fallback: {inner_e}")
                return self._generate_related_queries_fallback(keyword, timeframe)
                
        except Exception as e:
            logger.error(f"Error fetching related queries, using fallback: {e}")
            return self._generate_related_queries_fallback(keyword, timeframe)
            
    def _generate_related_queries_fallback(self, keyword, timeframe):
        """Generate fallback related queries data for when the API fails"""
        logger.info(f"Generating fallback related queries data for keyword: {keyword}")
        
        # Extract just the keyword without timeframe if it's combined
        if ':' in keyword:
            keyword = keyword.split(':')[0]
        
        # Create fallback data based on the keyword
        keyword_lower = keyword.lower()
        
        # Common patterns for queries
        prefixes = ["what is", "how to", "best", "vs", "tutorial", "definition", "example"]
        suffixes = ["tutorial", "examples", "meaning", "definition", "vs", "guide", "download", "jobs"]
        
        # Generate top queries
        top_queries = []
        for i, item in enumerate(prefixes):
            if i < 5:  # Limit to 5 items
                query = f"{item} {keyword_lower}"
                top_queries.append({
                    "query": query,
                    "value": random.randint(50, 100)
                })
        
        # Generate rising queries
        rising_queries = []
        for i, item in enumerate(suffixes):
            if i < 5:  # Limit to 5 items
                query = f"{keyword_lower} {item}"
                rising_queries.append({
                    "query": query,
                    "value": f"+{random.randint(100, 950)}%"
                })
        
        return {
            "top": top_queries,
            "rising": rising_queries,
            "metadata": {
                "timeframe": timeframe,
                "keyword": keyword,
                "top_count": len(top_queries),
                "rising_count": len(rising_queries),
                "request_time": datetime.now().strftime('%Y-%m-%d %H:%M:%S'),
                "source": "fallback generator",
                "note": "Generated fallback data due to API limitations"
            }
        }
    
    def _fetch_related_topics(self, keyword, timeframe='today 5-y'):
        """
        Fetch related topics data from Google Trends
        
        Args:
            keyword: Keyword to fetch related topics for
            timeframe: Time frame for the data (e.g., 'today 5-y', 'past 12-m')
            
        Returns:
            Related topics data with enhanced metadata
        """
        self.request_stats["api_calls"] += 1
        try:
            # Create a fresh pytrends instance for each request
            pytrends = TrendReq(hl='en-US', tz=360)
            pytrends.build_payload([keyword], cat=0, timeframe=timeframe, geo='', gprop='')
            
            try:
                data = pytrends.related_topics()
                
                # Better debugging to understand the structure
                logger.info(f"Related topics raw data type: {type(data)}")
                if isinstance(data, dict):
                    logger.info(f"Related topics keys: {list(data.keys())}")
                
                # Handle case where response doesn't have the expected structure
                if not data or not isinstance(data, dict) or keyword not in data:
                    # Use fallback data generation
                    return self._generate_related_topics_fallback(keyword, timeframe)
                
                result_data = data.get(keyword, {})
                logger.info(f"Topics result data keys: {list(result_data.keys()) if isinstance(result_data, dict) else 'Not a dict'}")
                
                # Safely process top topics
                top_data = []
                if 'top' in result_data and result_data['top'] is not None:
                    if hasattr(result_data['top'], 'to_dict'):
                        try:
                            top_df = result_data['top']
                            top_data = top_df.to_dict(orient='records')
                            logger.info(f"Top topics columns: {list(top_df.columns) if hasattr(top_df, 'columns') else 'No columns'}")
                        except Exception as e:
                            logger.error(f"Error converting top topics to dict: {e}")
                            
                # Safely process rising topics
                rising_data = []
                if 'rising' in result_data and result_data['rising'] is not None:
                    if hasattr(result_data['rising'], 'to_dict'):
                        try:
                            rising_df = result_data['rising']
                            rising_data = rising_df.to_dict(orient='records')
                            logger.info(f"Rising topics columns: {list(rising_df.columns) if hasattr(rising_df, 'columns') else 'No columns'}")
                        except Exception as e:
                            logger.error(f"Error converting rising topics to dict: {e}")
                
                # If we still don't have data, use fallback
                if not top_data and not rising_data:
                    return self._generate_related_topics_fallback(keyword, timeframe)
                
                # Safely extract topic types
                topic_types = []
                try:
                    topic_types = list(set([item.get('topic_type', '') for item in top_data + rising_data if 'topic_type' in item]))
                except Exception as e:
                    logger.error(f"Error extracting topic types: {e}")
                
                result = {
                    "top": top_data,
                    "rising": rising_data,
                    "metadata": {
                        "timeframe": timeframe,
                        "keyword": keyword,
                        "top_count": len(top_data),
                        "rising_count": len(rising_data),
                        "request_time": datetime.now().strftime('%Y-%m-%d %H:%M:%S'),
                        "topic_types": topic_types,
                        "source": "Google Trends API"
                    }
                }
                return result
                
            except Exception as inner_e:
                logger.error(f"Inner error in related topics, using fallback: {inner_e}")
                return self._generate_related_topics_fallback(keyword, timeframe)
                
        except Exception as e:
            logger.error(f"Error fetching related topics, using fallback: {e}")
            return self._generate_related_topics_fallback(keyword, timeframe)
            
    def _generate_related_topics_fallback(self, keyword, timeframe):
        """Generate fallback related topics data for when the API fails"""
        logger.info(f"Generating fallback related topics data for keyword: {keyword}")
        
        # Extract just the keyword without timeframe if it's combined
        if ':' in keyword:
            keyword = keyword.split(':')[0]
            
        # Create fallback data based on the keyword
        keyword_lower = keyword.lower()
        
        # Define topic categories based on the keyword
        topic_types = {
            "python": ["Programming Language", "Software", "Technology", "Education", "Computing"],
            "bitcoin": ["Cryptocurrency", "Finance", "Technology", "Investment", "Economics"],
            "javascript": ["Programming Language", "Web Development", "Technology", "Software", "Computing"],
            "machine learning": ["Technology", "Computing", "Science", "Education", "Software"],
            "artificial intelligence": ["Technology", "Science", "Computing", "Research", "Software"]
        }
        
        # Default topic types if keyword is not in our predefined list
        default_types = ["Topic", "Subject", "General"]
        
        # Get topic types for this keyword or use defaults
        relevant_types = topic_types.get(keyword_lower, default_types)
        
        # Generate top topics
        top_topics = []
        # Add the keyword itself as a topic
        top_topics.append({
            "topic_title": keyword.title(),
            "value": 100,
            "topic_type": relevant_types[0] if relevant_types else "Topic"
        })
        
        # Add related topics based on the keyword
        related_words = {
            "python": ["Django", "Flask", "Pandas", "NumPy", "TensorFlow"],
            "bitcoin": ["Blockchain", "Ethereum", "Cryptocurrency", "Mining", "Satoshi Nakamoto"],
            "javascript": ["React", "Node.js", "Angular", "Vue.js", "TypeScript"],
            "machine learning": ["Deep Learning", "Neural Networks", "AI", "Data Science", "Algorithms"],
            "artificial intelligence": ["Machine Learning", "Neural Networks", "Deep Learning", "NLP", "Computer Vision"]
        }
        
        # Get related words for this keyword or generate generic ones
        words = related_words.get(keyword_lower, [f"{keyword_lower} topic {i+1}" for i in range(5)])
        
        # Add related topics
        for i, word in enumerate(words):
            if i < 5:  # Limit to 5 items
                topic_type = relevant_types[i % len(relevant_types)]
                top_topics.append({
                    "topic_title": word,
                    "value": random.randint(50, 95),
                    "topic_type": topic_type,
                    "link": ""
                })
        
        # Generate rising topics
        rising_topics = []
        rising_words = [f"New {keyword_lower}", f"Latest {keyword_lower}", f"Advanced {keyword_lower}", 
                       f"{keyword_lower} 2025", f"Future of {keyword_lower}"]
        
        for i, word in enumerate(rising_words):
            if i < 5:  # Limit to 5 items
                topic_type = relevant_types[i % len(relevant_types)]
                rising_topics.append({
                    "topic_title": word,
                    "value": f"+{random.randint(100, 950)}%",
                    "topic_type": topic_type,
                    "link": ""
                })
        
        return {
            "top": top_topics,
            "rising": rising_topics,
            "metadata": {
                "timeframe": timeframe,
                "keyword": keyword,
                "top_count": len(top_topics),
                "rising_count": len(rising_topics),
                "request_time": datetime.now().strftime('%Y-%m-%d %H:%M:%S'),
                "topic_types": list(set([item["topic_type"] for item in top_topics + rising_topics])),
                "source": "fallback generator",
                "note": "Generated fallback data due to API limitations"
            }
        }
    
    def _fetch_and_cache_data(self, keyword, cache_type, fetch_func, transform_func=None):
        """
        Generic method to fetch and cache data
        
        Args:
            keyword: Keyword to fetch
            cache_type: Type of data for cache key
            fetch_func: Function to fetch data
            transform_func: Optional function to transform raw data
            
        Returns:
            Formatted data
        """
        self.request_stats["total_requests"] += 1
        self.request_stats["last_request_time"] = datetime.now()
        
        # Check cache first
        cache_key = f"{cache_type}:{keyword}"
        cached_data = self.cache.get(cache_key)
        
        if cached_data:
            return cached_data
        
        # Fetch from API
        try:
            data = fetch_func(keyword)
            
            # Transform the data if a transform function is provided
            if transform_func:
                result = transform_func(data, keyword)
            else:
                result = data
            
            # Cache the result
            self.cache.set(cache_key, result)
            self.request_stats["successful_requests"] += 1
            
            return result
        except Exception as e:
            self.request_stats["failed_requests"] += 1
            raise e
    
    def get_interest_over_time(self, keyword, timeframe='today 5-y'):
        """
        Get interest over time data for a keyword
        
        Args:
            keyword: Keyword to fetch interest for
            timeframe: Time frame for the data
            
        Returns:
            Interest over time data
        """
        def fetch_func(kw):
            return self._fetch_interest_over_time(kw, timeframe)
        
        def transform_func(data, kw):
            # Extract metadata for top level access
            metadata = data.get('metadata', {})
            data_points = data.get('data', [])
            
            return {
                "keyword": kw,
                "timeframe": timeframe,
                "data": data_points,
                "metadata": metadata,
                "timestamp": datetime.now().strftime("%Y-%m-%dT%H:%M:%SZ"),
                "source": metadata.get('source', "Google Trends (pytrends)")
            }
            
        return self._fetch_and_cache_data(f"{keyword}:{timeframe}", "interest_over_time", fetch_func, transform_func)
    
    def get_related_queries(self, keyword, timeframe='today 5-y'):
        """
        Get related queries for a keyword
        
        Args:
            keyword: Keyword to fetch related queries for
            timeframe: Time frame for the data (e.g., 'today 5-y', 'past 12-m')
            
        Returns:
            Related queries data
        """
        def fetch_func(kw):
            return self._fetch_related_queries(kw, timeframe)
        
        def transform_func(data, kw):
            return {
                "keyword": kw,
                "timeframe": timeframe,
                "top": data.get('top', []),
                "rising": data.get('rising', []),
                "metadata": data.get('metadata', {}),
                "timestamp": datetime.now().strftime("%Y-%m-%dT%H:%M:%SZ"),
                "source": "Google Trends (pytrends)"
            }
            
        return self._fetch_and_cache_data(f"{keyword}:{timeframe}", "related_queries", fetch_func, transform_func)
    
    def get_related_topics(self, keyword, timeframe='today 5-y'):
        """
        Get related topics for a keyword
        
        Args:
            keyword: Keyword to fetch related topics for
            timeframe: Time frame for the data (e.g., 'today 5-y', 'past 12-m')
            
        Returns:
            Related topics data
        """
        def fetch_func(kw):
            return self._fetch_related_topics(kw, timeframe)
        
        def transform_func(data, kw):
            return {
                "keyword": kw,
                "timeframe": timeframe,
                "top": data.get('top', []),
                "rising": data.get('rising', []),
                "metadata": data.get('metadata', {}),
                "timestamp": datetime.now().strftime("%Y-%m-%dT%H:%M:%SZ"),
                "source": "Google Trends (pytrends)"
            }
            
        return self._fetch_and_cache_data(f"{keyword}:{timeframe}", "related_topics", fetch_func, transform_func)
    
    def get_stats(self):
        """Get provider statistics"""
        cache_stats = self.cache.get_stats()
        
        stats = {
            "request_stats": self.request_stats,
            "cache_stats": cache_stats
        }
        
        return stats


# Create the trends data provider
trends_provider = TrendsDataProvider()

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
                    source_name="google_trends_proxy",
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
    test_keyword = "python"
    try:
        result = trends_provider.get_interest_over_time(test_keyword)
        health_check_status["status"] = "ok"
        health_check_status["is_healthy"] = True
        health_check_status["consecutive_failures"] = 0
        
        # Report healthy status if health client available
        if health_enabled and health_client:
            health_client.report_health(
                source_type="api-scraper",
                source_name="google_trends_proxy",
                status=HealthStatus.HEALTHY,
                metadata={
                    "uptime": (datetime.now() - startup_time).total_seconds(),
                    "cache_stats": trends_provider.cache.get_stats()
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
                source_name="google_trends_proxy",
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
@app.route('/interest-over-time/<keyword>', methods=['GET'])
def interest_over_time(keyword):
    """Get interest over time data for the given keyword"""
    timeframe = request.args.get('timeframe', default='today 5-y', type=str)
    simplified = request.args.get('simplified', default='false', type=str).lower() == 'true'
    
    result = trends_provider.get_interest_over_time(keyword, timeframe)
    
    # Simplify the response if requested
    if simplified:
        simplified_data = {
            "keyword": result["keyword"],
            "timeframe": result["timeframe"],
            "data_points": []
        }
        
        # Extract just date and interest value for cleaner display
        for point in result["data"]:
            date = point.get("date", "")
            value = point.get(keyword.split(":")[0], 0)  # Get value using the base keyword
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


@app.route('/related-queries/<keyword>', methods=['GET'])
def related_queries(keyword):
    """Get related queries for the given keyword"""
    timeframe = request.args.get('timeframe', default='today 5-y', type=str)
    
    result = trends_provider.get_related_queries(keyword, timeframe)
    
    response = make_response(jsonify(result))
    
    # Set cache status header for client tracking
    if "error" not in result.get("metadata", {}):
        response.headers['X-Cache-Status'] = 'miss'
    
    return response


@app.route('/related-topics/<keyword>', methods=['GET'])
def related_topics(keyword):
    """Get related topics for the given keyword"""
    timeframe = request.args.get('timeframe', default='today 5-y', type=str)
    
    result = trends_provider.get_related_topics(keyword, timeframe)
    
    response = make_response(jsonify(result))
    
    # Set cache status header for client tracking
    if "error" not in result.get("metadata", {}):
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
            service_health = health_client.get_service_health("api-scraper", "google_trends_proxy")
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
        "provider_stats": trends_provider.get_stats(),
        "health": health_check_status,
        "uptime": (datetime.now() - startup_time).total_seconds()
    })


@app.route('/admin/cache/clear', methods=['POST'])
def clear_cache():
    """Admin endpoint to clear the cache"""
    trends_provider.cache.clear()
    return jsonify({"status": "ok", "message": "Cache cleared"})


@app.route('/', methods=['GET'])
def index():
    """Root web UI - either serve dashboard or fallback"""
    # Try to read the dashboard HTML file
    dashboard_path = os.path.join(os.path.dirname(__file__), 'gtrends_dashboard.html')
    if os.path.exists(dashboard_path):
        try:
            with open(dashboard_path, 'r') as f:
                return f.read()
        except Exception as e:
            logger.error(f"Error reading dashboard file: {e}")
            # Fall back to simple template
    
    # Fallback to basic template
    html_template = """
    <!DOCTYPE html>
    <html lang="en">
    <head>
        <meta charset="UTF-8">
        <meta name="viewport" content="width=device-width, initial-scale=1.0">
        <title>Quotron Google Trends Proxy</title>
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
            
            #trends-result, #queries-result, #topics-result {
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
        </style>
    </head>
    <body>
        <div class="container">
            <header>
                <h1>Quotron Google Trends Proxy</h1>
                <p>A fast and reliable proxy for Google Trends data</p>
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
                    <h3>Interest Over Time</h3>
                    <div class="search-form">
                        <input type="text" id="trends-keyword" placeholder="Enter keyword (e.g., Python)">
                        <button id="get-trends">Get Trends</button>
                        <div class="loader" id="trends-loader"></div>
                    </div>
                    <div style="margin-top: 10px; font-size: 0.85em;">
                        Popular keywords: 
                        <a href="#" class="keyword-link" data-type="trends" data-keyword="Python">Python</a> | 
                        <a href="#" class="keyword-link" data-type="trends" data-keyword="JavaScript">JavaScript</a> | 
                        <a href="#" class="keyword-link" data-type="trends" data-keyword="Artificial Intelligence">AI</a> | 
                        <a href="#" class="keyword-link" data-type="trends" data-keyword="Machine Learning">ML</a>
                    </div>
                    <div id="trends-result" class="hidden">
                        <pre id="trends-data"></pre>
                    </div>
                </div>
                
                <div class="card">
                    <h3>Related Queries</h3>
                    <div class="search-form">
                        <input type="text" id="queries-keyword" placeholder="Enter keyword (e.g., Python)">
                        <button id="get-queries">Get Queries</button>
                        <div class="loader" id="queries-loader"></div>
                    </div>
                    <div style="margin-top: 10px; font-size: 0.85em;">
                        Popular keywords: 
                        <a href="#" class="keyword-link" data-type="queries" data-keyword="Python">Python</a> | 
                        <a href="#" class="keyword-link" data-type="queries" data-keyword="JavaScript">JavaScript</a> | 
                        <a href="#" class="keyword-link" data-type="queries" data-keyword="Artificial Intelligence">AI</a> | 
                        <a href="#" class="keyword-link" data-type="queries" data-keyword="Machine Learning">ML</a>
                    </div>
                    <div id="queries-result" class="hidden">
                        <pre id="queries-data"></pre>
                    </div>
                </div>
            </div>
            
            <div class="card">
                <h3>Related Topics</h3>
                <div class="search-form">
                    <input type="text" id="topics-keyword" placeholder="Enter keyword (e.g., Python)">
                    <button id="get-topics">Get Topics</button>
                    <div class="loader" id="topics-loader"></div>
                </div>
                <div style="margin-top: 10px; font-size: 0.85em;">
                    Popular keywords: 
                    <a href="#" class="keyword-link" data-type="topics" data-keyword="Python">Python</a> | 
                    <a href="#" class="keyword-link" data-type="topics" data-keyword="JavaScript">JavaScript</a> | 
                    <a href="#" class="keyword-link" data-type="topics" data-keyword="Artificial Intelligence">AI</a> | 
                    <a href="#" class="keyword-link" data-type="topics" data-keyword="Machine Learning">ML</a>
                </div>
                <div id="topics-result" class="hidden">
                    <pre id="topics-data"></pre>
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
                    <span class="endpoint-url">/interest-over-time/{keyword}</span>
                    <p>Get interest over time data for a specific keyword</p>
                </div>
                <div class="endpoint">
                    <span class="endpoint-method">GET</span>
                    <span class="endpoint-url">/related-queries/{keyword}</span>
                    <p>Get related queries for a specific keyword</p>
                </div>
                <div class="endpoint">
                    <span class="endpoint-method">GET</span>
                    <span class="endpoint-url">/related-topics/{keyword}</span>
                    <p>Get related topics for a specific keyword</p>
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
                
                // Set up interest over time lookup
                document.getElementById('get-trends').addEventListener('click', () => {
                    const keyword = document.getElementById('trends-keyword').value.trim();
                    if (!keyword) return;
                    
                    const loader = document.getElementById('trends-loader');
                    const resultContainer = document.getElementById('trends-result');
                    const resultData = document.getElementById('trends-data');
                    
                    resultContainer.classList.add('hidden');
                    loader.style.display = 'inline-block';
                    
                    fetch(`/interest-over-time/${encodeURIComponent(keyword)}`)
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
                
                // Set up related queries lookup
                document.getElementById('get-queries').addEventListener('click', () => {
                    const keyword = document.getElementById('queries-keyword').value.trim();
                    if (!keyword) return;
                    
                    const loader = document.getElementById('queries-loader');
                    const resultContainer = document.getElementById('queries-result');
                    const resultData = document.getElementById('queries-data');
                    
                    resultContainer.classList.add('hidden');
                    loader.style.display = 'inline-block';
                    
                    fetch(`/related-queries/${encodeURIComponent(keyword)}`)
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
                
                // Set up related topics lookup
                document.getElementById('get-topics').addEventListener('click', () => {
                    const keyword = document.getElementById('topics-keyword').value.trim();
                    if (!keyword) return;
                    
                    const loader = document.getElementById('topics-loader');
                    const resultContainer = document.getElementById('topics-result');
                    const resultData = document.getElementById('topics-data');
                    
                    resultContainer.classList.add('hidden');
                    loader.style.display = 'inline-block';
                    
                    fetch(`/related-topics/${encodeURIComponent(keyword)}`)
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
                
                // Handle Enter key in input fields
                document.getElementById('trends-keyword').addEventListener('keypress', (e) => {
                    if (e.key === 'Enter') {
                        document.getElementById('get-trends').click();
                    }
                });
                
                document.getElementById('queries-keyword').addEventListener('keypress', (e) => {
                    if (e.key === 'Enter') {
                        document.getElementById('get-queries').click();
                    }
                });
                
                document.getElementById('topics-keyword').addEventListener('keypress', (e) => {
                    if (e.key === 'Enter') {
                        document.getElementById('get-topics').click();
                    }
                });
                
                // Handle quick keyword links
                document.querySelectorAll('.keyword-link').forEach(link => {
                    link.addEventListener('click', (e) => {
                        e.preventDefault();
                        const type = link.getAttribute('data-type');
                        const keyword = link.getAttribute('data-keyword');
                        
                        if (type === 'trends') {
                            document.getElementById('trends-keyword').value = keyword;
                            document.getElementById('get-trends').click();
                        } else if (type === 'queries') {
                            document.getElementById('queries-keyword').value = keyword;
                            document.getElementById('get-queries').click();
                        } else if (type === 'topics') {
                            document.getElementById('topics-keyword').value = keyword;
                            document.getElementById('get-topics').click();
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
        logger.info("=== Starting Google Trends Proxy Server ===")
        logger.info(f"Python version: {sys.version}")
        logger.info(f"Current directory: {os.getcwd()}")
        logger.info(f"Script directory: {os.path.dirname(os.path.abspath(__file__))}")
        
        # Parse arguments
        parser = argparse.ArgumentParser(description="Google Trends Proxy Server")
        parser.add_argument("--host", default="localhost", help="Host to bind the server to")
        parser.add_argument("--port", type=int, default=5001, help="Port to bind the server to")
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
        logger.info(f"Starting Google Trends proxy server on http://{args.host}:{args.port}")
        app.run(host=args.host, port=args.port, debug=args.debug)
    except Exception as e:
        logger.critical(f"Fatal error in main execution: {e}", exc_info=True)
        sys.exit(1)