#!/usr/bin/env python3
"""
Test script for the pytrends library to understand how to properly use it
"""

import sys
import time
import logging
from pytrends.request import TrendReq

# Configure logging
logging.basicConfig(
    level=logging.INFO,
    format="%(asctime)s - %(name)s - %(levelname)s - %(message)s",
)
logger = logging.getLogger("pytrends-test")

def test_interest_over_time(keyword="python"):
    """Test interest over time functionality"""
    logger.info(f"Testing interest over time for keyword: {keyword}")
    
    try:
        pytrends = TrendReq(hl='en-US', tz=360)
        pytrends.build_payload([keyword], cat=0, timeframe='today 5-y', geo='', gprop='')
        
        # Get interest over time
        data = pytrends.interest_over_time()
        
        logger.info(f"Data type: {type(data)}")
        if hasattr(data, 'shape'):
            logger.info(f"Data shape: {data.shape}")
        if hasattr(data, 'columns'):
            logger.info(f"Data columns: {list(data.columns)}")
        if hasattr(data, 'index'):
            logger.info(f"Data index type: {type(data.index)}")
            logger.info(f"First few index values: {list(data.index)[:5]}")
            
        return data
    except Exception as e:
        logger.error(f"Error in interest over time: {e}")
        return None

def test_related_queries(keyword="python"):
    """Test related queries functionality"""
    logger.info(f"Testing related queries for keyword: {keyword}")
    
    try:
        pytrends = TrendReq(hl='en-US', tz=360)
        pytrends.build_payload([keyword], cat=0, timeframe='today 5-y', geo='', gprop='')
        
        # Get related queries
        data = pytrends.related_queries()
        
        logger.info(f"Data type: {type(data)}")
        if isinstance(data, dict):
            logger.info(f"Data keys: {list(data.keys())}")
            
            if keyword in data:
                result = data[keyword]
                logger.info(f"Result keys: {list(result.keys())}")
                
                for key in result:
                    if result[key] is not None:
                        logger.info(f"{key} type: {type(result[key])}")
                        if hasattr(result[key], 'shape'):
                            logger.info(f"{key} shape: {result[key].shape}")
                        if hasattr(result[key], 'columns'):
                            logger.info(f"{key} columns: {list(result[key].columns)}")
                    else:
                        logger.info(f"{key} is None")
        
        return data
    except Exception as e:
        logger.error(f"Error in related queries: {e}")
        return None

def test_related_topics(keyword="python"):
    """Test related topics functionality"""
    logger.info(f"Testing related topics for keyword: {keyword}")
    
    try:
        pytrends = TrendReq(hl='en-US', tz=360)
        pytrends.build_payload([keyword], cat=0, timeframe='today 5-y', geo='', gprop='')
        
        # Get related topics
        data = pytrends.related_topics()
        
        logger.info(f"Data type: {type(data)}")
        if isinstance(data, dict):
            logger.info(f"Data keys: {list(data.keys())}")
            
            if keyword in data:
                result = data[keyword]
                logger.info(f"Result keys: {list(result.keys())}")
                
                for key in result:
                    if result[key] is not None:
                        logger.info(f"{key} type: {type(result[key])}")
                        if hasattr(result[key], 'shape'):
                            logger.info(f"{key} shape: {result[key].shape}")
                        if hasattr(result[key], 'columns'):
                            logger.info(f"{key} columns: {list(result[key].columns)}")
                    else:
                        logger.info(f"{key} is None")
                
        return data
    except Exception as e:
        logger.error(f"Error in related topics: {e}")
        return None

def main():
    """Main function"""
    logger.info("Starting pytrends test")
    
    # Test with different keywords
    keywords = ["bitcoin", "python", "machine learning"]
    
    for keyword in keywords:
        logger.info(f"\n\n======= Testing keyword: {keyword} =======\n")
        
        logger.info("\n=== Interest Over Time ===")
        test_interest_over_time(keyword)
        
        # Wait to avoid rate limiting
        time.sleep(1)
        
        logger.info("\n=== Related Queries ===")
        test_related_queries(keyword)
        
        # Wait to avoid rate limiting
        time.sleep(1)
        
        logger.info("\n=== Related Topics ===")
        test_related_topics(keyword)
        
        # Wait between keywords
        time.sleep(2)
    
    logger.info("Pytrends test completed")

if __name__ == "__main__":
    main()