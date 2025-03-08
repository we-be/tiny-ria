#!/usr/bin/env python
"""
Integration test script for Quotron components.
This script tests the basic functionality of the API scraper, browser scraper,
authentication engine, and ingest pipeline.
"""

import os
import sys
import uuid
import json
import argparse
import logging
from datetime import datetime
from pathlib import Path

# Add project root to path for imports
project_root = Path(__file__).parent.parent.parent
sys.path.append(str(project_root))

# Configure logging
logging.basicConfig(
    level=logging.INFO,
    format="%(asctime)s - %(name)s - %(levelname)s - %(message)s",
)
logger = logging.getLogger(__name__)

def test_api_scraper():
    """Test the API scraper functionality."""
    logger.info("Testing API scraper...")
    
    # Run the Go API scraper with a test symbol
    # Check if we have an Alpha Vantage API key in the environment
    import subprocess
    import os
    
    api_key = os.getenv("ALPHA_VANTAGE_API_KEY", "demo")
    api_scraper_path = project_root / "api-scraper"
    
    # If using the demo key, we'll use a simpler command to avoid rate limits
    if api_key == "demo":
        logger.warning("Using demo API key - limited functionality available")
        cmd = ["./api-scraper", "--symbol", "IBM", "--api-key", "demo", "--json"]
    else:
        cmd = ["./api-scraper", "--symbol", "AAPL", "--api-key", api_key, "--json"]
    
    try:
        binary_path = api_scraper_path / "api-scraper"
        if not binary_path.exists():
            # Try to build the binary first
            build_cmd = ["go", "build", "-o", "api-scraper", "./cmd/main"]
            logger.info(f"Building API scraper: {' '.join(build_cmd)}")
            subprocess.run(build_cmd, cwd=api_scraper_path, check=True)
        
        logger.info(f"Running API scraper: ./api-scraper --symbol {'IBM' if api_key == 'demo' else 'AAPL'} --api-key [HIDDEN]")
        result = subprocess.run(cmd, cwd=api_scraper_path, capture_output=True, text=True)
        
        if result.returncode == 0:
            logger.info("API scraper test successful")
            # Truncate the output to avoid flooding the logs
            output_lines = result.stdout.strip().split('\n')
            if len(output_lines) > 4:
                truncated_output = '\n'.join(output_lines[:2] + ["..."] + output_lines[-2:])
                logger.info(f"Output (truncated): {truncated_output}")
            else:
                logger.info(f"Output: {result.stdout.strip()}")
        else:
            logger.warning(f"API scraper test failed with exit code {result.returncode}")
            logger.warning(f"Error: {result.stderr}")
            if api_key == "demo":
                logger.warning("This is expected with the demo key due to API limitations")
            else:
                logger.warning("API key may be invalid or rate limited")
    except Exception as e:
        logger.error(f"Error testing API scraper: {e}")

def test_browser_scraper():
    """Test the browser scraper functionality."""
    logger.info("Testing browser scraper...")
    
    try:
        # Import the scraper directly and run it in test mode
        sys.path.append(str(project_root / "browser-scraper" / "playwright"))
        from src.scraper import WebScraper
        
        scraper = WebScraper(headless=True)
        try:
            scraper.start()
            logger.info("Browser scraper initialized successfully")
            
            # We can't actually scrape without a valid URL, but we can test initialization
            logger.info("Browser scraper test successful")
        except Exception as e:
            logger.error(f"Error in browser scraper execution: {e}")
        finally:
            scraper.close()
    except Exception as e:
        logger.error(f"Error testing browser scraper: {e}")

def test_auth_engine():
    """Test the authentication engine functionality."""
    logger.info("Testing auth engine...")
    
    try:
        # Import the auth engine directly
        sys.path.append(str(project_root / "auth-engine"))
        from service.auth_service import (
            authenticate_user, 
            create_access_token,
            fake_users_db
        )
        
        # Test authentication with test user
        username = "testuser"
        password = "password123"
        
        user = authenticate_user(fake_users_db, username, password)
        if user:
            logger.info(f"Authentication successful for user: {username}")
            
            # Test token creation
            token_data = {"sub": username}
            token = create_access_token(data=token_data)
            logger.info(f"Token created successfully: {token[:20]}...")
            
            logger.info("Auth engine test successful")
        else:
            logger.error(f"Authentication failed for user: {username}")
    except Exception as e:
        logger.error(f"Error testing auth engine: {e}")

def test_ingest_pipeline():
    """Test the ingest pipeline functionality."""
    logger.info("Testing ingest pipeline...")
    
    try:
        # Import the schema directly without the validator (which uses pandas)
        sys.path.append(str(project_root / "ingest-pipeline"))
        from schemas.finance_schema import StockQuote, Exchange, DataSource
        
        # Create test data
        test_quote = {
            "symbol": "AAPL",
            "price": 150.25,
            "change": 2.5,
            "change_percent": 1.2,
            "volume": 12345678,
            "timestamp": datetime.now(),
            "exchange": "NYSE",
            "source": "api-scraper"
        }
        
        # Validate the data using the schema directly
        quote = StockQuote(**test_quote)
        
        if quote:
            logger.info(f"Quote validation successful: {quote.symbol} ${quote.price}")
            logger.info("Ingest pipeline test successful")
        else:
            logger.error("Quote validation failed")
    except Exception as e:
        logger.error(f"Error testing ingest pipeline: {e}")

def test_events_system():
    """Test the events system functionality."""
    logger.info("Testing events system...")
    
    try:
        # Import the events system components
        sys.path.append(str(project_root / "events"))
        from schemas.event_schema import StockQuoteEvent
        
        # Create a test event
        event = StockQuoteEvent(
            event_id=str(uuid.uuid4()),
            source="test-script",
            data={
                "symbol": "AAPL",
                "price": 150.25,
                "change": 2.5,
                "change_percent": 1.69,
                "volume": 12345678,
            },
            metadata={
                "environment": "test",
            }
        )
        
        # Validate that the event can be serialized to JSON
        event_json = json.dumps(event.model_dump(), default=str)
        logger.info(f"Event serialized successfully: {event_json[:100]}...")
        logger.info("Events system test successful")
    except Exception as e:
        logger.error(f"Error testing events system: {e}")

def main():
    """Run all tests."""
    parser = argparse.ArgumentParser(description="Quotron integration tests")
    parser.add_argument("--api", action="store_true", help="Test API scraper")
    parser.add_argument("--browser", action="store_true", help="Test browser scraper")
    parser.add_argument("--auth", action="store_true", help="Test auth engine")
    parser.add_argument("--ingest", action="store_true", help="Test ingest pipeline")
    parser.add_argument("--events", action="store_true", help="Test events system")
    parser.add_argument("--all", action="store_true", help="Test all components")
    
    args = parser.parse_args()
    
    # If no specific tests are specified, test all
    if not any([args.api, args.browser, args.auth, args.ingest, args.events, args.all]):
        args.all = True
    
    if args.all or args.api:
        test_api_scraper()
    
    if args.all or args.browser:
        test_browser_scraper()
    
    if args.all or args.auth:
        test_auth_engine()
    
    if args.all or args.ingest:
        test_ingest_pipeline()
    
    if args.all or args.events:
        test_events_system()
    
    logger.info("All tests completed")

if __name__ == "__main__":
    main()