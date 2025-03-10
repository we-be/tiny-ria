#!/usr/bin/env python3
"""Test script to check health of data sources and generate a report."""

import os
import sys
import json
import time
import requests
from datetime import datetime

def check_yfinance_proxy():
    """Check the health of the YFinance Proxy."""
    proxy_url = os.environ.get("YFINANCE_PROXY_URL", "http://localhost:5000")
    health_endpoint = f"{proxy_url}/health"
    
    print(f"Checking YFinance proxy at {health_endpoint}...")
    try:
        response = requests.get(health_endpoint, timeout=5)
        if response.status_code == 200:
            health_data = response.json()
            print(f"✅ YFinance proxy is healthy!")
            print(f"Status: {health_data.get('status')}")
            print(f"Uptime: {health_data.get('uptime', 0):.2f} seconds")
            print(f"Last check: {health_data.get('last_check')}")
            return True
        else:
            print(f"❌ YFinance proxy returned status code {response.status_code}")
            return False
    except requests.RequestException as e:
        print(f"❌ Failed to connect to YFinance proxy: {e}")
        return False

def check_scheduler():
    """Check if the scheduler is running."""
    print("Checking scheduler status...")
    try:
        import subprocess
        result = subprocess.run(["pgrep", "-f", "scheduler"], capture_output=True, text=True)
        if result.returncode == 0:
            pid = result.stdout.strip()
            print(f"✅ Scheduler is running with PID {pid}")
            return True
        else:
            print("❌ Scheduler is not running")
            return False
    except Exception as e:
        print(f"❌ Failed to check scheduler status: {e}")
        return False

def generate_report():
    """Generate a health report for all data sources."""
    report = {
        "generated_at": datetime.now().strftime("%Y-%m-%d %H:%M:%S"),
        "sources": {
            "yfinance_proxy": {
                "status": "unknown",
                "details": {}
            },
            "scheduler": {
                "status": "unknown",
                "details": {}
            }
        }
    }
    
    # Check YFinance Proxy
    proxy_url = os.environ.get("YFINANCE_PROXY_URL", "http://localhost:5000")
    health_endpoint = f"{proxy_url}/health"
    try:
        response = requests.get(health_endpoint, timeout=5)
        if response.status_code == 200:
            report["sources"]["yfinance_proxy"]["status"] = "healthy"
            report["sources"]["yfinance_proxy"]["details"] = response.json()
        else:
            report["sources"]["yfinance_proxy"]["status"] = "unhealthy"
            report["sources"]["yfinance_proxy"]["details"] = {
                "error": f"Status code {response.status_code}"
            }
    except requests.RequestException as e:
        report["sources"]["yfinance_proxy"]["status"] = "failed"
        report["sources"]["yfinance_proxy"]["details"] = {
            "error": str(e)
        }
    
    # Check Scheduler
    try:
        import subprocess
        result = subprocess.run(["pgrep", "-f", "scheduler"], capture_output=True, text=True)
        if result.returncode == 0:
            pid = result.stdout.strip()
            report["sources"]["scheduler"]["status"] = "running"
            report["sources"]["scheduler"]["details"] = {
                "pid": pid
            }
        else:
            report["sources"]["scheduler"]["status"] = "stopped"
    except Exception as e:
        report["sources"]["scheduler"]["status"] = "unknown"
        report["sources"]["scheduler"]["details"] = {
            "error": str(e)
        }
    
    # Write report to file
    report_path = "health_report.json"
    with open(report_path, "w") as f:
        json.dump(report, f, indent=2)
    
    print(f"Health report written to {report_path}")
    return report

def main():
    """Main entry point for the script."""
    print("Data Source Health Check Tool")
    print("============================")
    print(f"Current time: {datetime.now().strftime('%Y-%m-%d %H:%M:%S')}")
    
    # Check environment variables
    proxy_url = os.environ.get("YFINANCE_PROXY_URL", "http://localhost:5000")
    print(f"Using YFINANCE_PROXY_URL: {proxy_url}")
    
    # Run checks
    yfinance_ok = check_yfinance_proxy()
    scheduler_ok = check_scheduler()
    
    # Generate report
    report = generate_report()
    
    # Determine overall status
    if yfinance_ok and scheduler_ok:
        print("\n✅ All services are running correctly!")
    else:
        print("\n❌ Some services are not running correctly!")
        if not yfinance_ok:
            print("   - YFinance proxy is not healthy")
        if not scheduler_ok:
            print("   - Scheduler is not running")
        print("\nTo restart all services, run: ./start_all.sh")
    
    return 0

if __name__ == "__main__":
    sys.exit(main())