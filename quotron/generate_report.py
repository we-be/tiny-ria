#!/usr/bin/env python3
"""Generate a diagnostic report for data sources"""

import psycopg2
import json
import os
import pandas as pd
from datetime import datetime
from psycopg2.extras import RealDictCursor

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

def generate_diagnostics_report():
    """Generate a diagnostics report for all data sources"""
    conn = connect_to_db()
    
    try:
        # Get health data from database
        with conn.cursor(cursor_factory=RealDictCursor) as cur:
            cur.execute("""
                SELECT * FROM data_source_health
                ORDER BY source_type, source_name
            """)
            health_data = pd.DataFrame(cur.fetchall()) if cur.rowcount > 0 else pd.DataFrame()
    except Exception as e:
        print(f"Error retrieving health data: {e}")
        return
    finally:
        conn.close()
    
    # Create the report file path
    report_path = os.path.join(os.path.dirname(os.path.abspath(__file__)), "diagnostics_report.md")
    
    # Get current time
    now = datetime.now().strftime("%Y-%m-%d %H:%M:%S")
    
    # Start the report with a header
    report = f"""# Data Source Diagnostics Report
Generated at: {now}

## Overview
"""
    
    # Add overall statistics
    healthy_count = len(health_data[health_data['status'] == 'healthy'])
    degraded_count = len(health_data[health_data['status'].isin(['degraded', 'limited'])])
    failed_count = len(health_data[health_data['status'].isin(['failed', 'error'])])
    total_count = len(health_data)
    
    health_score = (healthy_count + (degraded_count * 0.5)) / total_count * 100 if total_count > 0 else 0
    
    report += f"""
- **Total Sources**: {total_count}
- **Healthy**: {healthy_count} ({healthy_count/total_count*100:.1f}%)
- **Degraded**: {degraded_count} ({degraded_count/total_count*100:.1f}%)
- **Failed**: {failed_count} ({failed_count/total_count*100:.1f}%)
- **Health Score**: {health_score:.1f}%

## Health Summary

| Source | Type | Status | Last Success | Age | Error Count |
|--------|------|--------|--------------|-----|-------------|
"""
    
    # Add each source to the report
    for _, source in health_data.iterrows():
        status = source['status']
        status_emoji = "✅" if status == 'healthy' else "⚠️" if status in ['degraded', 'limited'] else "❌"
        
        # Format last success time
        if pd.notna(source.get('last_success')):
            last_success = source['last_success'].strftime("%Y-%m-%d %H:%M:%S")
            # Make sure both datetimes are timezone aware or naive
            try:
                now = datetime.now(source['last_success'].tzinfo)
                age = (now - source['last_success']).total_seconds() / 60
                age_text = f"{int(age)} min" if age < 60 else f"{int(age/60)} hours"
            except:
                # Fallback if datetime comparison fails
                age_text = "Unknown"
        else:
            last_success = "Never"
            age_text = "N/A"
        
        report += f"| {source['source_name']} | {source['source_type']} | {status_emoji} {status} | {last_success} | {age_text} | {source['error_count']} |\n"
    
    # Write the report to a file
    with open(report_path, "w") as f:
        f.write(report)
    
    print(f"Diagnostics report written to {report_path}")
    return report_path

if __name__ == "__main__":
    generate_diagnostics_report()