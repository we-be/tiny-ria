#!/usr/bin/env python3
"""
Format Google Trends data in a more readable way

Usage: python3 format_trends.py {keyword} [--short]
Example: python3 format_trends.py bitcoin --short
"""

import sys
import json
import argparse
import urllib.request
from datetime import datetime

def format_interest_over_time(keyword, short=False):
    """Format interest over time data in a readable way"""
    # Fetch data from the local proxy
    url = f"http://localhost:5001/interest-over-time/{urllib.parse.quote(keyword)}"
    print(f"Fetching data from: {url}")
    
    try:
        response = urllib.request.urlopen(url)
        data = json.loads(response.read())
        
        # Get the metadata
        metadata = data.get('metadata', {})
        source = data.get('source', 'Unknown')
        timeframe = data.get('timeframe', 'Unknown')
        
        # Print header
        print(f"\n=== Interest Over Time for '{keyword}' ===")
        print(f"Timeframe: {timeframe}")
        print(f"Source: {source}")
        print(f"Data points: {metadata.get('data_points', len(data.get('data', [])))}")
        if 'start_date' in metadata and 'end_date' in metadata:
            print(f"Date range: {metadata['start_date']} to {metadata['end_date']}")
        print("\n")
        
        # Determine number of points to show
        raw_data = data.get('data', [])
        if not raw_data:
            print("No data available.")
            return
            
        if short:
            # For short format, show only points with significant changes or quarterly
            # We'll sample every 3 months (quarterly) data
            step = max(1, len(raw_data) // 20)  # Show around 20 points max
            data_to_show = raw_data[::step]
        else:
            data_to_show = raw_data
        
        # Extract base keyword without timeframe
        base_keyword = keyword.split(':')[0] if ':' in keyword else keyword
        
        # Print formatted data
        print(f"{'DATE':<12} | {'INTEREST':<10}")
        print("-" * 25)
        
        for point in data_to_show:
            date = point.get('date', 'N/A')
            value = point.get(base_keyword, 0)
            print(f"{date:<12} | {value:<10.1f}")
        
        print("\n")
    except Exception as e:
        print(f"Error: {e}")

def format_related_queries(keyword, short=False):
    """Format related queries data in a readable way"""
    # Fetch data from the local proxy
    url = f"http://localhost:5001/related-queries/{urllib.parse.quote(keyword)}"
    print(f"Fetching data from: {url}")
    
    try:
        response = urllib.request.urlopen(url)
        data = json.loads(response.read())
        
        # Print header
        print(f"\n=== Related Queries for '{keyword}' ===")
        print(f"Source: {data.get('source', 'Unknown')}")
        print(f"Timestamp: {data.get('timestamp', 'Unknown')}")
        print("\n")
        
        # Process top queries
        top_queries = data.get('top', [])
        if top_queries:
            print("TOP QUERIES:")
            print(f"{'QUERY':<30} | {'VALUE':<10}")
            print("-" * 45)
            
            # Limit if short mode is enabled
            if short and len(top_queries) > 5:
                top_queries = top_queries[:5]
                
            for i, query in enumerate(top_queries):
                if 'query' in query and 'value' in query:
                    print(f"{query['query']:<30} | {query['value']}")
            
            print("\n")
                
        # Process rising queries
        rising_queries = data.get('rising', [])
        if rising_queries:
            print("RISING QUERIES:")
            print(f"{'QUERY':<30} | {'VALUE':<10}")
            print("-" * 45)
            
            # Limit if short mode is enabled
            if short and len(rising_queries) > 5:
                rising_queries = rising_queries[:5]
                
            for i, query in enumerate(rising_queries):
                if 'query' in query and 'value' in query:
                    print(f"{query['query']:<30} | {query['value']}")
            
            print("\n")
        
        if not top_queries and not rising_queries:
            print("No related queries data available.")
    except Exception as e:
        print(f"Error: {e}")

def format_related_topics(keyword, short=False):
    """Format related topics data in a readable way"""
    # Fetch data from the local proxy
    url = f"http://localhost:5001/related-topics/{urllib.parse.quote(keyword)}"
    print(f"Fetching data from: {url}")
    
    try:
        response = urllib.request.urlopen(url)
        data = json.loads(response.read())
        
        # Print header
        print(f"\n=== Related Topics for '{keyword}' ===")
        print(f"Source: {data.get('source', 'Unknown')}")
        print(f"Timestamp: {data.get('timestamp', 'Unknown')}")
        print("\n")
        
        # Process top topics
        top_topics = data.get('top', [])
        if top_topics:
            print("TOP TOPICS:")
            print(f"{'TOPIC':<30} | {'TYPE':<15} | {'VALUE':<10}")
            print("-" * 60)
            
            # Limit if short mode is enabled
            if short and len(top_topics) > 5:
                top_topics = top_topics[:5]
                
            for i, topic in enumerate(top_topics):
                topic_title = topic.get('topic_title', 'N/A')
                topic_type = topic.get('topic_type', 'N/A')
                value = topic.get('value', 'N/A')
                print(f"{topic_title:<30} | {topic_type:<15} | {value}")
            
            print("\n")
                
        # Process rising topics
        rising_topics = data.get('rising', [])
        if rising_topics:
            print("RISING TOPICS:")
            print(f"{'TOPIC':<30} | {'TYPE':<15} | {'VALUE':<10}")
            print("-" * 60)
            
            # Limit if short mode is enabled
            if short and len(rising_topics) > 5:
                rising_topics = rising_topics[:5]
                
            for i, topic in enumerate(rising_topics):
                topic_title = topic.get('topic_title', 'N/A')
                topic_type = topic.get('topic_type', 'N/A')
                value = topic.get('value', 'N/A')
                print(f"{topic_title:<30} | {topic_type:<15} | {value}")
            
            print("\n")
        
        if not top_topics and not rising_topics:
            print("No related topics data available.")
    except Exception as e:
        print(f"Error: {e}")

def main():
    # Parse command line arguments
    parser = argparse.ArgumentParser(description='Format Google Trends data in a readable way')
    parser.add_argument('keyword', help='Keyword to search for')
    parser.add_argument('--type', choices=['interest', 'queries', 'topics', 'all'], default='all',
                        help='Type of data to retrieve (default: all)')
    parser.add_argument('--short', action='store_true', help='Show shortened output')
    
    args = parser.parse_args()
    
    # Process based on type
    if args.type == 'interest' or args.type == 'all':
        format_interest_over_time(args.keyword, args.short)
        
    if args.type == 'queries' or args.type == 'all':
        format_related_queries(args.keyword, args.short)
        
    if args.type == 'topics' or args.type == 'all':
        format_related_topics(args.keyword, args.short)

if __name__ == "__main__":
    main()