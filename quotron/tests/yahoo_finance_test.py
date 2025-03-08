#!/usr/bin/env python3
"""
Test the Yahoo Finance integration with the API scraper.
"""

import os
import subprocess
import sys
import json
from pathlib import Path

# Add project root to path for imports
project_root = Path(__file__).parent.parent
api_scraper_path = project_root / "api-scraper"

def test_yahoo_finance():
    """Test the Yahoo Finance integration."""
    print("Testing Yahoo Finance integration...")
    
    # Run the API scraper with Yahoo Finance
    cmd = [str(api_scraper_path / "api-scraper"), "--yahoo", "--symbol", "AAPL", "--json"]
    
    print(f"Running command: {' '.join(cmd)}")
    result = subprocess.run(cmd, capture_output=True, text=True)
    
    if result.returncode == 0:
        print("Yahoo Finance test successful")
        
        # Parse the JSON output
        output_lines = result.stdout.strip().split('\n')
        for line in output_lines:
            if line.startswith('{'):
                try:
                    data = json.loads(line)
                    if "symbol" in data:
                        print(f"Got quote for {data['symbol']}: ${data['price']}")
                    elif "indexName" in data:
                        print(f"Got market data for {data['indexName']}: {data['value']}")
                except json.JSONDecodeError:
                    pass
    else:
        print(f"Yahoo Finance test failed with exit code {result.returncode}")
        print(f"Error: {result.stderr}")

if __name__ == "__main__":
    test_yahoo_finance()