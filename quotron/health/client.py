#!/usr/bin/env python3
"""
Client library for the health monitoring service.
This allows Python services to report and query health statuses.
"""

import requests
import json
import time
from datetime import datetime
from enum import Enum
from typing import Dict, Any, Optional, List, Union

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
        self.session.headers.update({
            "Content-Type": "application/json"
        })
    
    def report_health(self, source_type: str, source_name: str, status: str, 
                     source_detail: Optional[str] = None, 
                     response_time_ms: Optional[int] = None, 
                     error_message: Optional[str] = None, 
                     metadata: Optional[Dict[str, Any]] = None) -> bool:
        """Report health status to the health service
        
        Args:
            source_type (str): Type of the service (e.g., 'api-scraper')
            source_name (str): Name of the service (e.g., 'yahoo_finance_proxy')
            status (str): Health status (use HealthStatus enum)
            source_detail (str, optional): Human-readable description
            response_time_ms (int, optional): Response time in milliseconds
            error_message (str, optional): Error message if any
            metadata (dict, optional): Additional metadata
            
        Returns:
            bool: True if the report was accepted, False otherwise
        """
        url = f"{self.service_url}/health"
        
        report = {
            "source_type": source_type,
            "source_name": source_name,
            "status": status,
            "last_check": datetime.now().isoformat()
        }
        
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
    
    def get_service_health(self, source_type: str, source_name: str) -> Optional[Dict[str, Any]]:
        """Get health status for a specific service
        
        Args:
            source_type (str): Type of the service
            source_name (str): Name of the service
            
        Returns:
            dict or None: Health report for the service or None if not found
        """
        url = f"{self.service_url}/health/{source_type}/{source_name}"
        
        try:
            response = self.session.get(url, timeout=5)
            if response.status_code == 200:
                return response.json()
            return None
        except Exception as e:
            print(f"Error getting health status: {e}")
            return None
    
    def get_all_health(self) -> List[Dict[str, Any]]:
        """Get health status for all services
        
        Returns:
            list: List of health reports
        """
        url = f"{self.service_url}/health"
        
        try:
            response = self.session.get(url, timeout=5)
            if response.status_code == 200:
                return response.json()
            return []
        except Exception as e:
            print(f"Error getting all health statuses: {e}")
            return []
    
    def get_system_health(self) -> Optional[Dict[str, Any]]:
        """Get overall system health
        
        Returns:
            dict or None: System health or None if error
        """
        url = f"{self.service_url}/health/system"
        
        try:
            response = self.session.get(url, timeout=5)
            if response.status_code == 200:
                return response.json()
            return None
        except Exception as e:
            print(f"Error getting system health: {e}")
            return None
    
    def record_request(self, source_type: str, source_name: str, 
                      url: str, success: bool, 
                      response_time_ms: Optional[int] = None,
                      error_message: Optional[str] = None) -> bool:
        """Record a request (convenience method for scrapers and API calls)
        
        Args:
            source_type (str): Type of service
            source_name (str): Name of service
            url (str): URL that was requested
            success (bool): Whether the request was successful
            response_time_ms (int, optional): Response time in milliseconds
            error_message (str, optional): Error message if the request failed
            
        Returns:
            bool: True if the report was accepted
        """
        status = HealthStatus.HEALTHY if success else HealthStatus.FAILED
        
        metadata = {
            "url": url,
            "timestamp": datetime.now().isoformat()
        }
        
        return self.report_health(
            source_type=source_type,
            source_name=source_name,
            status=status,
            response_time_ms=response_time_ms,
            error_message=error_message,
            metadata=metadata
        )

# Helper functions for common tasks

def measure_request_time(func):
    """Decorator to measure request time and report health
    
    Example:
        @measure_request_time
        def fetch_data(url, client, source_type, source_name):
            response = requests.get(url)
            return response.json()
    """
    def wrapper(*args, **kwargs):
        start_time = time.time()
        client = kwargs.get('client')
        source_type = kwargs.get('source_type')
        source_name = kwargs.get('source_name')
        url = kwargs.get('url', 'unknown')
        
        try:
            result = func(*args, **kwargs)
            elapsed_ms = int((time.time() - start_time) * 1000)
            
            if client and source_type and source_name:
                client.record_request(
                    source_type=source_type,
                    source_name=source_name,
                    url=url,
                    success=True,
                    response_time_ms=elapsed_ms
                )
            
            return result
        except Exception as e:
            elapsed_ms = int((time.time() - start_time) * 1000)
            
            if client and source_type and source_name:
                client.record_request(
                    source_type=source_type,
                    source_name=source_name,
                    url=url,
                    success=False,
                    response_time_ms=elapsed_ms,
                    error_message=str(e)
                )
            
            raise
    
    return wrapper