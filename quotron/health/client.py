#!/usr/bin/env python3
"""
Simplified client library for the health monitoring service.
This allows Python services to report and query health statuses.
"""

import requests
import time
from datetime import datetime
from enum import Enum
from typing import Dict, Any, Optional, Callable, List, Union

class HealthStatus(str, Enum):
    """Health status values"""
    HEALTHY = "healthy"
    DEGRADED = "degraded"
    FAILED = "failed"
    LIMITED = "limited"
    UNKNOWN = "unknown"

class HealthClient:
    """Client for the health monitoring service"""
    
    def __init__(self, service_url="http://localhost:8085"):
        """Initialize the health client
        
        Args:
            service_url (str): URL of the health service
        """
        self.service_url = service_url
        self.session = requests.Session()
        self.session.headers.update({"Content-Type": "application/json"})
    
    def report_health(self, source_type: str, source_name: str, status: str, 
                     source_detail: Optional[str] = None, 
                     response_time_ms: Optional[int] = None, 
                     error_message: Optional[str] = None, 
                     metadata: Optional[Dict[str, Any]] = None) -> bool:
        """Report health status to the health service"""
        url = f"{self.service_url}/health"
        
        # Prepare report
        report = {
            "source_type": source_type,
            "source_name": source_name,
            "status": status,
            "last_check": datetime.now().isoformat(),
        }
        
        # Add optional fields if provided
        if source_detail:
            report["source_detail"] = source_detail
        if response_time_ms is not None:
            report["response_time_ms"] = response_time_ms
        if error_message:
            report["error_message"] = error_message
        if metadata:
            report["metadata"] = metadata
        
        try:
            response = self.session.post(url, json=report, timeout=5)
            return response.status_code in (200, 202)
        except Exception as e:
            print(f"Error reporting health: {e}")
            return False
    
    def get_health(self, source_type: str = None, source_name: str = None) -> Optional[Union[Dict, List]]:
        """Get health status for a service or all services
        
        Args:
            source_type (str, optional): Type of source (e.g. api-scraper)
            source_name (str, optional): Name of source (e.g. yahoo_finance)
            
        Returns:
            dict, list, or None: Health report, list of reports, or None if error
        """
        if source_type and source_name:
            url = f"{self.service_url}/health/{source_type}/{source_name}"
        elif source_type == "system":
            url = f"{self.service_url}/health/system"
        else:
            url = f"{self.service_url}/health"
        
        try:
            response = self.session.get(url, timeout=5)
            if response.status_code == 200:
                return response.json()
            return None
        except Exception as e:
            print(f"Error getting health: {e}")
            return None
    
    def monitor_health(self, source_type: str, source_name: str, source_detail: Optional[str] = None) -> Callable:
        """Decorator to monitor health of a function"""
        def decorator(func):
            def wrapper(*args, **kwargs):
                start_time = time.time()
                try:
                    result = func(*args, **kwargs)
                    elapsed_ms = int((time.time() - start_time) * 1000)
                    
                    # Report successful health
                    self.report_health(
                        source_type=source_type,
                        source_name=source_name,
                        source_detail=source_detail,
                        status=HealthStatus.HEALTHY,
                        response_time_ms=elapsed_ms,
                    )
                    
                    return result
                except Exception as e:
                    elapsed_ms = int((time.time() - start_time) * 1000)
                    
                    # Report failed health
                    self.report_health(
                        source_type=source_type,
                        source_name=source_name,
                        source_detail=source_detail,
                        status=HealthStatus.FAILED,
                        response_time_ms=elapsed_ms,
                        error_message=str(e),
                    )
                    
                    # Re-raise the exception
                    raise
            return wrapper
        return decorator

def measure_request_time(func):
    """Decorator to measure request time in milliseconds"""
    def wrapper(*args, **kwargs):
        start_time = time.time()
        result = func(*args, **kwargs)
        elapsed_ms = int((time.time() - start_time) * 1000)
        return result, elapsed_ms
    return wrapper