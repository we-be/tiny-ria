#!/usr/bin/env python3
"""
SP500 Scraper - Scrapes S&P 500 component data from slickcharts.com
"""

import json
import logging
import os
import sys
from datetime import datetime
from pathlib import Path
import pandas as pd
import requests
from bs4 import BeautifulSoup
from io import StringIO

# Configure logging
logging.basicConfig(
    level=logging.INFO,
    format="%(asctime)s - %(name)s - %(levelname)s - %(message)s",
)
logger = logging.getLogger(__name__)

# URL for S&P 500 components
SP500_URL = "https://www.slickcharts.com/sp500"

# User agent to avoid being blocked
HEADERS = {
    "User-Agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36",
}

# Define a function to safely convert percentage strings to float
def clean_percentage(x):
    if pd.isna(x):
        return 0.0
    
    # Convert to string if it's not already
    x = str(x)
    
    # Print problematic values for debugging
    if '(' in x or '--' in x:
        print(f"Converting problematic value: {x}")
    
    # Handle parentheses (negative numbers)
    if '(' in x and ')' in x:
        x = x.replace('(', '-').replace(')', '')
    
    # Handle double negatives
    x = x.replace('--', '')
    
    # Remove percent sign
    x = x.replace('%', '')
    
    # Strip whitespace
    x = x.strip()
    
    try:
        return float(x)
    except ValueError:
        print(f"Failed to convert: {x}")
        return 0.0

def scrape_sp500():
    """Scrape S&P 500 components from slickcharts.com"""
    logger.info("Scraping S&P 500 components from %s", SP500_URL)
    
    try:
        response = requests.get(SP500_URL, headers=HEADERS, timeout=30)
        response.raise_for_status()
    except requests.exceptions.RequestException as e:
        logger.error("Failed to fetch S&P 500 data: %s", e)
        return None
    
    # Parse HTML
    soup = BeautifulSoup(response.text, "html.parser")
    
    # Find the table with the components
    table = soup.find("table", class_="table table-hover table-borderless table-sm")
    if not table:
        logger.error("S&P 500 table not found on page")
        return None
    
    # Extract data using pandas
    df = pd.read_html(StringIO(str(table)))[0]
    
    # Format the data
    df.columns = [col.strip() for col in df.columns]
    
    # Print columns for debugging
    print("Columns:", df.columns.tolist())
    
    # Convert percentage strings to float values
    if "Weight" in df.columns:
        print("Sample weight values:", df["Weight"].head(5).tolist())
        df["Weight"] = df["Weight"].apply(clean_percentage)
    
    # Different column name variations for percent change
    pct_chg_col = None
    pct_chg_cols = ['%Chg', 'Chg%', '% Chg', 'Pct Chg', 'Change%']
    for col in pct_chg_cols:
        if col in df.columns:
            print(f"Found percent change column: {col}")
            print("Sample values:", df[col].head(5).tolist())
            df[col] = df[col].apply(clean_percentage)
            pct_chg_col = col
            break
    
    # Calculate total market cap for allocation percentages
    total_weight = df["Weight"].sum()
    
    # Build the holdings data
    holdings = []
    
    for _, row in df.iterrows():
        holding = {
            "ticker": row["Symbol"],
            "position_name": row["Company"],
            "allocation": float(row["Weight"]),
            "price": float(row["Price"]),
            "chg": float(row["Chg"]),
        }
        
        # Add percent change if the column exists
        if pct_chg_col:
            holding["pct_chg"] = float(row[pct_chg_col])
        else:
            holding["pct_chg"] = 0.0  # Default value
            
        holdings.append(holding)
    
    # Create sector groupings
    # Note: In a real implementation, we would classify each company by sector
    # For now, we'll return empty sectors since slickcharts doesn't provide sector data
    
    # Build the investment model data
    model = {
        "provider": "SlickCharts",
        "model_name": "S&P 500 Index",
        "detail_level": "full",
        "source": "slickcharts.com",
        "fetched_at": datetime.utcnow().strftime("%Y-%m-%dT%H:%M:%SZ"),
        "holdings": holdings,
        "sectors": []  # Empty for now
    }
    
    return model

def save_model_to_json(model, output_path):
    """Save the model data to a JSON file"""
    if not model:
        return False
    
    try:
        with open(output_path, "w", encoding="utf-8") as f:
            json.dump(model, f, indent=2)
        logger.info("Model data saved to %s", output_path)
        return True
    except Exception as e:
        logger.error("Failed to save model data: %s", e)
        return False

def main():
    """Main entry point"""
    output_dir = Path(os.path.dirname(os.path.dirname(os.path.abspath(__file__)))) / "output"
    output_dir.mkdir(exist_ok=True)
    
    output_path = output_dir / f"sp500_model_{datetime.now().strftime('%Y%m%d')}.json"
    
    model = scrape_sp500()
    if model:
        if save_model_to_json(model, output_path):
            logger.info("S&P 500 model data scraped successfully")
            print(output_path)  # Output the path for use by other processes
            return 0
    
    logger.error("Failed to scrape S&P 500 model data")
    return 1

if __name__ == "__main__":
    sys.exit(main())