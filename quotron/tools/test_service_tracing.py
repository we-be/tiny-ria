#!/usr/bin/env python3
"""
Test script for the service tracing functionality.
This script generates sample trace data and inserts it into the database
to demonstrate the service trace visualization in the dashboard.
"""

import sys
import os
import time
import uuid
import json
import random
import psycopg2
import datetime
from psycopg2.extras import Json

# Configuration
DB_HOST = os.environ.get("DB_HOST", "localhost")
DB_PORT = os.environ.get("DB_PORT", "5432")
DB_NAME = os.environ.get("DB_NAME", "quotron")
DB_USER = os.environ.get("DB_USER", "quotron")
DB_PASSWORD = os.environ.get("DB_PASSWORD", "quotron")

# Services and operations to simulate
SERVICES = {
    "scheduler": [
        "stock_quote_job",
        "market_index_job",
        "data_source_health_job"
    ],
    "api-service": [
        "/api/quote/{symbol}",
        "/api/index/{index}",
        "/api/health"
    ],
    "yahoo_finance_proxy": [
        "/quote/{symbol}",
        "/quote/historical/{symbol}",
        "/cache/stats"
    ],
    "database": [
        "INSERT",
        "SELECT",
        "UPDATE"
    ],
    "browser-scraper": [
        "fetch_sp500",
        "fetch_dow30",
        "fetch_nasdaq100"
    ]
}

# Function to connect to the database
def get_db_connection():
    """Connect to the PostgreSQL database."""
    try:
        conn = psycopg2.connect(
            host=DB_HOST,
            port=DB_PORT,
            database=DB_NAME,
            user=DB_USER,
            password=DB_PASSWORD
        )
        return conn
    except Exception as e:
        print(f"Error connecting to database: {e}")
        sys.exit(1)

# Function to generate a random trace with multiple spans
def generate_trace():
    """Generate a complete trace with multiple spans."""
    trace_id = str(uuid.uuid4())
    
    # Decide which flow to simulate
    flow_type = random.choice(["stock_quote", "market_index", "health_check"])
    
    # Create spans based on the flow type
    spans = []
    
    # Base time for this trace
    base_time = datetime.datetime.now() - datetime.timedelta(minutes=random.randint(0, 60))
    
    if flow_type == "stock_quote":
        # Scheduler initiates a stock quote job
        scheduler_span = create_span(
            trace_id=trace_id,
            parent_id=None,
            name=SERVICES["scheduler"][0],  # stock_quote_job
            service="scheduler",
            start_time=base_time,
            duration_ms=random.randint(100, 500),
            status="success"
        )
        spans.append(scheduler_span)
        
        # API service processes the request
        api_span = create_span(
            trace_id=trace_id,
            parent_id=scheduler_span["span_id"],
            name=SERVICES["api-service"][0],  # /api/quote/{symbol}
            service="api-service",
            start_time=base_time + datetime.timedelta(milliseconds=scheduler_span["duration_ms"] // 2),
            duration_ms=random.randint(50, 200),
            status="success"
        )
        spans.append(api_span)
        
        # Yahoo Finance proxy fetches the data
        yahoo_span = create_span(
            trace_id=trace_id,
            parent_id=api_span["span_id"],
            name=SERVICES["yahoo_finance_proxy"][0],  # /quote/{symbol}
            service="yahoo_finance_proxy",
            start_time=base_time + datetime.timedelta(milliseconds=scheduler_span["duration_ms"] // 2 + api_span["duration_ms"] // 2),
            duration_ms=random.randint(100, 400),
            status="success" if random.random() > 0.2 else "error"
        )
        spans.append(yahoo_span)
        
        # Database stores the data
        if yahoo_span["status"] == "success":
            db_span = create_span(
                trace_id=trace_id,
                parent_id=api_span["span_id"],
                name=SERVICES["database"][0],  # INSERT
                service="database",
                start_time=base_time + datetime.timedelta(milliseconds=scheduler_span["duration_ms"] // 2 + api_span["duration_ms"] // 2 + yahoo_span["duration_ms"]),
                duration_ms=random.randint(10, 50),
                status="success"
            )
            spans.append(db_span)
    
    elif flow_type == "market_index":
        # Scheduler initiates a market index job
        scheduler_span = create_span(
            trace_id=trace_id,
            parent_id=None,
            name=SERVICES["scheduler"][1],  # market_index_job
            service="scheduler",
            start_time=base_time,
            duration_ms=random.randint(100, 500),
            status="success"
        )
        spans.append(scheduler_span)
        
        # API service processes the request
        api_span = create_span(
            trace_id=trace_id,
            parent_id=scheduler_span["span_id"],
            name=SERVICES["api-service"][1],  # /api/index/{index}
            service="api-service",
            start_time=base_time + datetime.timedelta(milliseconds=scheduler_span["duration_ms"] // 2),
            duration_ms=random.randint(50, 200),
            status="success"
        )
        spans.append(api_span)
        
        # Browser scraper fetches the data
        browser_span = create_span(
            trace_id=trace_id,
            parent_id=api_span["span_id"],
            name=random.choice(SERVICES["browser-scraper"]),
            service="browser-scraper",
            start_time=base_time + datetime.timedelta(milliseconds=scheduler_span["duration_ms"] // 2 + api_span["duration_ms"] // 2),
            duration_ms=random.randint(500, 2000),
            status="success" if random.random() > 0.1 else "error"
        )
        spans.append(browser_span)
        
        # Database stores the data
        if browser_span["status"] == "success":
            db_span = create_span(
                trace_id=trace_id,
                parent_id=api_span["span_id"],
                name=SERVICES["database"][0],  # INSERT
                service="database",
                start_time=base_time + datetime.timedelta(milliseconds=scheduler_span["duration_ms"] // 2 + api_span["duration_ms"] // 2 + browser_span["duration_ms"]),
                duration_ms=random.randint(10, 50),
                status="success"
            )
            spans.append(db_span)
    
    elif flow_type == "health_check":
        # Scheduler initiates a health check
        scheduler_span = create_span(
            trace_id=trace_id,
            parent_id=None,
            name=SERVICES["scheduler"][2],  # data_source_health_job
            service="scheduler",
            start_time=base_time,
            duration_ms=random.randint(50, 200),
            status="success"
        )
        spans.append(scheduler_span)
        
        # API service health check
        api_span = create_span(
            trace_id=trace_id,
            parent_id=scheduler_span["span_id"],
            name=SERVICES["api-service"][2],  # /api/health
            service="api-service",
            start_time=base_time + datetime.timedelta(milliseconds=scheduler_span["duration_ms"] // 2),
            duration_ms=random.randint(20, 100),
            status="success"
        )
        spans.append(api_span)
        
        # Yahoo Finance proxy health check
        yahoo_span = create_span(
            trace_id=trace_id,
            parent_id=scheduler_span["span_id"],
            name=SERVICES["yahoo_finance_proxy"][2],  # /cache/stats
            service="yahoo_finance_proxy",
            start_time=base_time + datetime.timedelta(milliseconds=scheduler_span["duration_ms"] // 2 + 10),
            duration_ms=random.randint(10, 50),
            status="success" if random.random() > 0.05 else "error"
        )
        spans.append(yahoo_span)
        
        # Database health check
        db_span = create_span(
            trace_id=trace_id,
            parent_id=scheduler_span["span_id"],
            name=SERVICES["database"][1],  # SELECT
            service="database",
            start_time=base_time + datetime.timedelta(milliseconds=scheduler_span["duration_ms"] // 2 + 20),
            duration_ms=random.randint(5, 30),
            status="success"
        )
        spans.append(db_span)
    
    return spans

def create_span(trace_id, parent_id, name, service, start_time, duration_ms, status):
    """Create a single span for a trace."""
    span_id = str(uuid.uuid4())
    end_time = start_time + datetime.timedelta(milliseconds=duration_ms)
    
    error_message = None
    if status == "error":
        error_types = {
            "scheduler": ["Job execution failed", "Timeout waiting for response", "Invalid configuration"],
            "api-service": ["Internal server error", "Bad gateway", "Service unavailable"],
            "yahoo_finance_proxy": ["API rate limit exceeded", "Connection refused", "Invalid response format"],
            "database": ["Connection error", "Constraint violation", "Timeout waiting for lock"],
            "browser-scraper": ["Page structure changed", "Element not found", "JavaScript error"]
        }
        error_message = random.choice(error_types.get(service, ["Unknown error"]))
    
    # Create metadata based on the service type
    metadata = {
        "span_type": "http" if service in ["api-service", "yahoo_finance_proxy"] else "internal",
        "environment": "production"
    }
    
    if service == "scheduler":
        metadata["job_type"] = name.split("_")[0]
        metadata["trigger"] = "cron"
    elif service == "api-service":
        metadata["http_method"] = "GET"
        metadata["http_path"] = name.replace("{symbol}", "AAPL").replace("{index}", "S&P500")
    elif service == "yahoo_finance_proxy":
        metadata["cache_hit"] = random.choice([True, False])
        metadata["data_source"] = "yahoo_finance_api"
    elif service == "database":
        metadata["query_type"] = name
        metadata["rows_affected"] = random.randint(1, 100)
    
    return {
        "trace_id": trace_id,
        "span_id": span_id,
        "parent_id": parent_id,
        "name": name,
        "service": service,
        "start_time": start_time,
        "end_time": end_time,
        "duration_ms": duration_ms,
        "status": status,
        "error_message": error_message,
        "metadata": json.dumps(metadata)
    }

def insert_spans(conn, spans):
    """Insert spans into the database."""
    with conn.cursor() as cur:
        for span in spans:
            cur.execute(
                """INSERT INTO service_traces 
                (trace_id, parent_id, name, service, start_time, end_time, duration_ms, status, error_message, metadata) 
                VALUES (%s, %s, %s, %s, %s, %s, %s, %s, %s, %s)""",
                (
                    span["trace_id"], 
                    span["parent_id"],
                    span["name"],
                    span["service"],
                    span["start_time"],
                    span["end_time"],
                    span["duration_ms"],
                    span["status"],
                    span["error_message"],
                    span["metadata"]
                )
            )
    conn.commit()

def main():
    """Main function to generate and insert trace data."""
    # Parse command line arguments
    import argparse
    parser = argparse.ArgumentParser(description='Generate and insert trace data for testing.')
    parser.add_argument('--traces', type=int, default=20, help='Number of traces to generate')
    args = parser.parse_args()
    
    # Connect to the database
    conn = get_db_connection()
    
    print(f"Generating {args.traces} traces...")
    
    # Generate and insert traces
    for i in range(args.traces):
        spans = generate_trace()
        insert_spans(conn, spans)
        print(f"Inserted trace {i+1}/{args.traces} with {len(spans)} spans")
        time.sleep(0.1)  # Small delay to prevent overwhelming the database
    
    # Close the connection
    conn.close()
    
    print(f"Done! {args.traces} traces with their spans have been inserted into the database.")
    print("You can now view them in the dashboard on the 'Service Traces' tab.")

if __name__ == "__main__":
    main()