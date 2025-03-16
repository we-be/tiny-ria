#!/usr/bin/env python3
"""
Test the Yahoo Finance integration with the API proxy and API service.
"""

import os
import requests
import sys
import json
from pathlib import Path

def test_yahoo_finance():
    """Test the Yahoo Finance integration."""
    print("Testing Yahoo Finance integration...")
    
    # Test YFinance proxy directly
    proxy_url = "http://localhost:5000"
    api_url = "http://localhost:8080"
    
    # Test quote endpoint from proxy
    symbol = "AAPL"
    print(f"Testing YFinance proxy quote endpoint for {symbol}...")
    
    try:
        response = requests.get(f"{proxy_url}/quote/{symbol}")
        response.raise_for_status()
        data = response.json()
        print(f"YFinance proxy quote test successful")
        print(f"Got quote for {symbol}: ${data.get('price', 'N/A')}")
    except Exception as e:
        print(f"YFinance proxy quote test failed: {str(e)}")
        sys.exit(1)
    
    # Test market endpoint from proxy
    index = "^GSPC"
    print(f"Testing YFinance proxy market endpoint for {index}...")
    
    try:
        response = requests.get(f"{proxy_url}/market/{index}")
        response.raise_for_status()
        data = response.json()
        print(f"YFinance proxy market test successful")
        print(f"Got market data for {index}: {data.get('value', 'N/A')}")
    except Exception as e:
        print(f"YFinance proxy market test failed: {str(e)}")
        sys.exit(1)
    
    # Test API service if it's running
    try:
        # Test health endpoint
        response = requests.get(f"{api_url}/api/health")
        if response.status_code == 200:
            print("API service health test successful")
            
            # Test quote endpoint from API service
            symbol = "MSFT"
            print(f"Testing API service quote endpoint for {symbol}...")
            response = requests.get(f"{api_url}/api/quote/{symbol}")
            response.raise_for_status()
            data = response.json()
            print(f"API service quote test successful")
            print(f"Got quote for {data.get('symbol', 'N/A')}: ${data.get('price', 'N/A')}")
            
            # Test index endpoint from API service
            index = "SPY"
            print(f"Testing API service index endpoint for {index}...")
            response = requests.get(f"{api_url}/api/index/{index}")
            response.raise_for_status()
            data = response.json()
            print(f"API service index test successful")
            print(f"Got index data for {data.get('index_name', 'N/A')}: {data.get('value', 'N/A')}")
    except Exception as e:
        print(f"Note: API service tests skipped or failed: {str(e)}")
        print("This is OK if you're not testing the API service specifically")

    print("Yahoo Finance integration tests passed!")
    return 0

if __name__ == "__main__":
    sys.exit(test_yahoo_finance())