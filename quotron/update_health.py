#!/usr/bin/env python3
"""Script to update data source health status based on data in the database"""

import psycopg2
import json
import os
from datetime import datetime

def connect_to_db():
    """Connect to the database"""
    host = os.environ.get("DB_HOST", "localhost")
    port = os.environ.get("DB_PORT", "5432")
    dbname = os.environ.get("DB_NAME", "quotron")
    user = os.environ.get("DB_USER", "quotron")
    password = os.environ.get("DB_PASSWORD", "quotron")
    
    connection = psycopg2.connect(
        host=host,
        port=port,
        dbname=dbname,
        user=user,
        password=password
    )
    
    return connection

def update_source_health(conn, source_name, status, last_success=None, records=None):
    """Update the health status of a source"""
    print(f"Updating {source_name} status to {status}")
    with conn.cursor() as cur:
        try:
            metadata = {}
            if records is not None:
                metadata["record_count"] = records
            
            metadata_json = json.dumps(metadata) if metadata else None
            
            if last_success:
                update_query = """
                    UPDATE data_source_health
                    SET status = %s,
                        last_check = NOW(),
                        last_success = %s,
                        metadata = %s
                    WHERE source_name = %s
                """
                cur.execute(update_query, (status, last_success, metadata_json, source_name))
            else:
                update_query = """
                    UPDATE data_source_health
                    SET status = %s,
                        last_check = NOW(),
                        metadata = %s
                    WHERE source_name = %s
                """
                cur.execute(update_query, (status, metadata_json, source_name))
            
            conn.commit()
            print(f"Successfully updated {source_name} health status")
        except Exception as e:
            conn.rollback()
            print(f"Failed to update {source_name} health status: {e}")

def check_data_sources():
    """Check all data sources based on database records"""
    conn = connect_to_db()
    
    try:
        # Check Alpha Vantage and Yahoo Finance health by looking at database records
        with conn.cursor() as cur:
            # Check for recent records from API Scraper
            cur.execute("""
                SELECT COUNT(*) as record_count, MAX(timestamp) as last_record
                FROM stock_quotes
                WHERE source = 'api-scraper'
                AND timestamp > NOW() - INTERVAL '24 hours'
            """)
            alpha_vantage_data = cur.fetchone()
            
            if alpha_vantage_data and alpha_vantage_data[0] > 0:
                # Alpha Vantage has recent records
                update_source_health(conn, "alpha_vantage", 
                                   "healthy", 
                                   alpha_vantage_data[1],
                                   records=alpha_vantage_data[0])
            else:
                print("No recent Alpha Vantage records found")
            
            # Check for recent records from Yahoo Finance (direct)
            cur.execute("""
                SELECT COUNT(*) as record_count, MAX(timestamp) as last_record
                FROM stock_quotes
                WHERE source = 'api-scraper'
                AND timestamp > NOW() - INTERVAL '24 hours' 
            """)
            yahoo_data = cur.fetchone()
            
            if yahoo_data and yahoo_data[0] > 0:
                # Yahoo Finance has recent records
                update_source_health(conn, "yahoo_finance", 
                                   "healthy", 
                                   yahoo_data[1],
                                   records=yahoo_data[0])
            else:
                print("No recent Yahoo Finance records found")
            
            # Check browser scrapers
            cur.execute("""
                SELECT COUNT(*) as record_count, MAX(timestamp) as last_record
                FROM market_indices
                WHERE source = 'browser-scraper'
                AND timestamp > NOW() - INTERVAL '3 days'
            """)
            browser_data = cur.fetchone()
            
            if browser_data and browser_data[0] > 0:
                # Browser scrapers have recent records
                update_source_health(conn, "slickcharts", 
                                   "healthy", 
                                   browser_data[1],
                                   records=browser_data[0])
            else:
                print("No recent browser scraper records found")
    except Exception as e:
        print(f"Error checking database for source health: {e}")
    finally:
        conn.close()

if __name__ == "__main__":
    print("Checking data source health...")
    check_data_sources()
    print("Done checking data source health")