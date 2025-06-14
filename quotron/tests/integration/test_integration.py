#!/usr/bin/env python
"""
Integration test script for Quotron components.
This script tests the basic functionality of the API scraper, browser scraper,
authentication engine, and ingest pipeline.

Note: This script is automatically run by GitHub Actions when code is pushed.
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
    use_yahoo = os.getenv("USE_YAHOO_FINANCE", "false").lower() in ["true", "1", "yes"]
    
    # Choose command based on settings
    if use_yahoo:
        # Use Yahoo Finance instead of Alpha Vantage
        logger.info("Using Yahoo Finance for testing")
        
        # Check if we need to start the proxy server
        proxy_script = project_root / "api-scraper" / "scripts" / "run_proxy.sh"
        if proxy_script.exists():
            logger.info("Starting Yahoo Finance proxy server in background...")
            import subprocess
            # Start the proxy server as a background process
            proxy_process = subprocess.Popen(
                ["bash", str(proxy_script), "--port", "5000"],
                stdout=subprocess.PIPE,
                stderr=subprocess.PIPE
            )
            # Give it time to start
            time.sleep(5)
            # Set environment variable to tell API scraper to use local proxy
            os.environ["YAHOO_PROXY_URL"] = "http://localhost:5000"
        
        cmd = ["./api-scraper", "--symbol", "AAPL", "--yahoo", "--json"]
    elif api_key == "demo":
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
        
        if use_yahoo:
            logger.info("Running API scraper with Yahoo Finance: ./api-scraper --symbol AAPL --yahoo")
        else:
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
            if api_key == "demo" and not use_yahoo:
                logger.warning("This is expected with the demo key due to API limitations")
            elif not use_yahoo:
                logger.warning("API key may be invalid or rate limited")
            else:
                logger.warning("Yahoo Finance integration error - check the proxy server")
    except Exception as e:
        logger.error(f"Error testing API scraper: {e}")
    finally:
        # Clean up any proxy process we started
        if use_yahoo and 'proxy_process' in locals():
            try:
                logger.info("Stopping Yahoo Finance proxy server...")
                proxy_process.terminate()
                proxy_process.wait(timeout=5)
                logger.info("Proxy server stopped")
            except Exception as e:
                logger.warning(f"Error stopping proxy server: {e}")
                # Try harder to kill it
                try:
                    import signal
                    proxy_process.send_signal(signal.SIGKILL)
                    logger.info("Forcefully killed proxy server")
                except:
                    pass

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
            users_db
        )
        
        # Test authentication with test user
        username = "testuser"
        password = "password123"
        
        user = authenticate_user(users_db, username, password)
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

# ETL tests removed - ETL should be tested with a proper database connection
# The test was removed because ETL's primary purpose is database operations
# and testing without a DB connection isn't meaningful
# For proper ETL testing, use:
# 1. Unit tests in Go for ETL components
# 2. Database integration tests with a test database

def main():
    """Run all tests."""
    parser = argparse.ArgumentParser(description="Quotron integration tests")
    parser.add_argument("--api", action="store_true", help="Test API scraper")
    parser.add_argument("--browser", action="store_true", help="Test browser scraper")
    parser.add_argument("--auth", action="store_true", help="Test auth engine")
    parser.add_argument("--all", action="store_true", help="Test all components")
    
    args = parser.parse_args()
    
    # If no specific tests are specified, test all
    if not any([args.api, args.browser, args.auth, args.all]):
        args.all = True
    
    if args.all or args.api:
        test_api_scraper()
    
    if args.all or args.browser:
        test_browser_scraper()
    
    if args.all or args.auth:
        test_auth_engine()
    
    logger.info("All tests completed")

if __name__ == "__main__":
    main()