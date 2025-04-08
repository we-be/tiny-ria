#!/usr/bin/env python3
"""
Test script for Economic Factors Proxy
"""

import requests
import json
import sys

BASE_URL = "http://localhost:5002"

def pretty_print(data):
    """Print JSON data in a readable format"""
    print(json.dumps(data, indent=2))

def test_health():
    """Test the health endpoint"""
    print("\n=== Testing Health Endpoint ===")
    response = requests.get(f"{BASE_URL}/health")
    if response.status_code == 200:
        print("Health check successful!")
        pretty_print(response.json())
    else:
        print(f"Failed with status code {response.status_code}")
        print(response.text)

def test_indicators():
    """Test the indicators endpoint"""
    print("\n=== Testing Indicators Endpoint ===")
    response = requests.get(f"{BASE_URL}/indicators")
    if response.status_code == 200:
        data = response.json()
        print(f"Found {len(data.get('indicators', []))} indicators")
        # Just print the first few indicators
        pretty_print(data['indicators'][:3] if 'indicators' in data else data)
    else:
        print(f"Failed with status code {response.status_code}")
        print(response.text)

def test_indicator(indicator="GDP"):
    """Test a specific indicator endpoint"""
    print(f"\n=== Testing Indicator Endpoint for {indicator} ===")
    response = requests.get(f"{BASE_URL}/indicator/{indicator}", params={"period": "5y"})
    if response.status_code == 200:
        data = response.json()
        print(f"Successfully retrieved data for {indicator}")
        # Only print essential info from the first few data points
        if 'data' in data and len(data['data']) > 0:
            print(f"Data points: {len(data['data'])}")
            print("First few data points:")
            for point in data['data'][:3]:
                print(f"  Date: {point.get('date')}, Value: {point.get('value')}")
        else:
            pretty_print(data)
    else:
        print(f"Failed with status code {response.status_code}")
        print(response.text)

def test_summary():
    """Test the summary endpoint"""
    print("\n=== Testing Summary Endpoint ===")
    response = requests.get(f"{BASE_URL}/summary")
    if response.status_code == 200:
        data = response.json()
        print("Economic Summary:")
        
        # Print overall health
        if 'overall' in data:
            overall = data['overall']
            print(f"Overall US Economic Health: {overall.get('health', 'N/A')}")
            print(f"Score: {overall.get('score', 'N/A')}")
            print(f"Indicators trending up: {overall.get('trending_up', 'N/A')}")
            print(f"Indicators trending down: {overall.get('trending_down', 'N/A')}")
            
        # Print summary data for key indicators
        if 'summary' in data:
            print("\nKey indicators:")
            for indicator, info in data['summary'].items():
                trend_symbol = "↑" if info.get('trend') == "up" else "↓" if info.get('trend') == "down" else "→"
                print(f"  {info.get('name', indicator)}: {info.get('value', 'N/A')} {info.get('unit', '')} {trend_symbol} {info.get('change_pct', 0)}%")
    else:
        print(f"Failed with status code {response.status_code}")
        print(response.text)

def main():
    """Run all tests"""
    print("=== Testing Economic Factors Proxy ===")
    
    try:
        test_health()
        test_indicators()
        test_indicator("GDP")
        test_indicator("UNRATE")
        test_summary()
        
        print("\n=== All tests completed! ===")
    except requests.exceptions.ConnectionError:
        print("ERROR: Could not connect to the proxy server. Make sure it's running at", BASE_URL)
        sys.exit(1)
    except Exception as e:
        print(f"ERROR: {e}")
        sys.exit(1)

if __name__ == "__main__":
    main()