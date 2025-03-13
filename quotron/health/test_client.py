#!/usr/bin/env python3
"""
Test client for the health monitoring service.
This allows testing the Python client library.
"""

import argparse
import time
import json
from datetime import datetime
from client import HealthClient, HealthStatus

def main():
    """Main entry point for the test client"""
    parser = argparse.ArgumentParser(description="Health monitoring test client")
    parser.add_argument("--url", default="http://localhost:8085", help="Health service URL")
    parser.add_argument("--type", default="test-client", help="Source type")
    parser.add_argument("--name", default="python-client", help="Source name")
    parser.add_argument("--action", default="report", choices=["report", "get", "all", "system"], 
                        help="Action to perform")
    parser.add_argument("--status", default="healthy", choices=["healthy", "degraded", "failed", "limited", "unknown"],
                        help="Health status (for report action)")
    args = parser.parse_args()
    
    # Create client
    client = HealthClient(args.url)
    
    # Perform the requested action
    if args.action == "report":
        # Report health
        metadata = {
            "test": True,
            "timestamp": datetime.now().isoformat(),
            "python_version": "3.x"
        }
        
        success = client.report_health(
            source_type=args.type,
            source_name=args.name,
            status=args.status,
            source_detail="Python test client for health monitoring",
            response_time_ms=50,
            metadata=metadata
        )
        
        if success:
            print("Health reported successfully")
        else:
            print("Failed to report health")
    
    elif args.action == "get":
        # Get health for a specific service
        report = client.get_service_health(args.type, args.name)
        if report:
            print(f"Health for {args.type}/{args.name}: {report['status']}")
            print(f"Last check: {report['last_check']}")
            if 'last_success' in report and report['last_success']:
                print(f"Last success: {report['last_success']}")
            print(f"Response time: {report.get('response_time_ms', 'N/A')} ms")
            print(f"Error count: {report.get('error_count', 0)}")
            if 'error_message' in report and report['error_message']:
                print(f"Error message: {report['error_message']}")
            print("Metadata:", json.dumps(report.get('metadata', {}), indent=2))
        else:
            print(f"No health report found for {args.type}/{args.name}")
    
    elif args.action == "all":
        # Get all health statuses
        reports = client.get_all_health()
        print(f"Found {len(reports)} health reports:")
        for report in reports:
            print(f"- {report['source_type']}/{report['source_name']}: {report['status']}")
    
    elif args.action == "system":
        # Get system health
        system_health = client.get_system_health()
        if system_health:
            print(f"System health score: {system_health['health_score']:.2f}%")
            print(f"Services: {system_health['total_services']} total, "
                  f"{system_health['healthy_count']} healthy, "
                  f"{system_health['degraded_count']} degraded, "
                  f"{system_health['failed_count']} failed")
        else:
            print("Failed to get system health")

if __name__ == "__main__":
    main()