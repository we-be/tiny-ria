#!/usr/bin/env python3
"""
Benchmark for Python ETL pipeline
"""

import random
import time
import datetime
import multiprocessing
from enum import Enum
from typing import List, Dict, Any

# Mock models for Python
class DataSource(str, Enum):
    API_SCRAPER = "api-scraper"
    BROWSER_SCRAPER = "browser-scraper"
    MANUAL = "manual"

class Exchange(str, Enum):
    NYSE = "NYSE"
    NASDAQ = "NASDAQ"
    AMEX = "AMEX"
    OTC = "OTC"
    OTHER = "OTHER"

class StockQuote:
    def __init__(self, symbol, price, change, change_percent, volume, timestamp, exchange, source):
        self.symbol = symbol
        self.price = price
        self.change = change
        self.change_percent = change_percent
        self.volume = volume
        self.timestamp = timestamp
        self.exchange = exchange
        self.source = source

class MarketIndex:
    def __init__(self, name, value, change, change_percent, timestamp, source):
        self.name = name
        self.value = value
        self.change = change
        self.change_percent = change_percent
        self.timestamp = timestamp
        self.source = source

# Helper functions to generate random data
def generate_random_quotes(count: int) -> List[StockQuote]:
    symbols = ["AAPL", "MSFT", "GOOG", "AMZN", "META"]
    exchanges = [Exchange.NYSE, Exchange.NASDAQ]
    now = datetime.datetime.now()
    
    quotes = []
    for i in range(count):
        symbol = random.choice(symbols)
        price = 50.0 + random.random() * 450.0
        change = -10.0 + random.random() * 20.0
        change_percent = (change / (price - change)) * 100.0
        volume = random.randint(1000, 10000000)
        exchange = random.choice(exchanges)
        
        quotes.append(StockQuote(
            symbol=symbol,
            price=price,
            change=change,
            change_percent=change_percent,
            volume=volume,
            timestamp=now,
            exchange=exchange,
            source=DataSource.API_SCRAPER
        ))
    
    return quotes

def generate_random_indices(count: int) -> List[MarketIndex]:
    names = ["S&P 500", "Dow Jones", "NASDAQ"]
    now = datetime.datetime.now()
    
    indices = []
    for i in range(count):
        name = names[i % len(names)]
        value = 1000.0 + random.random() * 29000.0
        change = -100.0 + random.random() * 200.0
        change_percent = (change / (value - change)) * 100.0
        
        indices.append(MarketIndex(
            name=name,
            value=value,
            change=change,
            change_percent=change_percent,
            timestamp=now,
            source=DataSource.API_SCRAPER
        ))
    
    return indices

# Process quotes worker
def process_quotes(quotes: List[StockQuote], result_queue):
    for i, quote in enumerate(quotes):
        # Simulate validation and processing
        if i % 10 == 0:
            time.sleep(0.001)  # 1ms delay
    
    result_queue.put(True)

# Process indices worker
def process_indices(indices: List[MarketIndex], result_queue):
    for i, index in enumerate(indices):
        # Simulate validation and processing
        if i % 5 == 0:
            time.sleep(0.001)  # 1ms delay
    
    result_queue.put(True)

def main():
    # Set random seed for reproducibility
    random.seed(int(time.time()))
    
    batch_sizes = [10, 100, 1000]
    
    for size in batch_sizes:
        # Generate random data
        quotes = generate_random_quotes(size)
        indices = generate_random_indices(size // 10)
        
        # Simulate processing time
        start_time = time.time()
        
        # Use multiprocessing for parallel processing
        result_queue = multiprocessing.Queue()
        
        # Process quotes in a separate process
        quotes_process = multiprocessing.Process(
            target=process_quotes,
            args=(quotes, result_queue)
        )
        
        # Process indices in a separate process
        indices_process = multiprocessing.Process(
            target=process_indices,
            args=(indices, result_queue)
        )
        
        # Start processes
        quotes_process.start()
        indices_process.start()
        
        # Wait for processes to complete
        result_queue.get()
        result_queue.get()
        
        # Stop processes
        quotes_process.join()
        indices_process.join()
        
        duration = time.time() - start_time
        items_per_second = (len(quotes) + len(indices)) / duration
        
        print(f"Batch size {size}: processed {len(quotes) + len(indices)} items in {duration:.6f}s ({items_per_second:.2f} items/sec)")
    
    print("Benchmark complete")

if __name__ == "__main__":
    main()