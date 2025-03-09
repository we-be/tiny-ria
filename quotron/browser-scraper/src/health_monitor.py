import os
import time
import json
import psycopg2
import psycopg2.extras
from datetime import datetime
from enum import Enum
import logging

# Configure logging
logging.basicConfig(
    level=logging.INFO,
    format='%(asctime)s - %(name)s - %(levelname)s - %(message)s'
)
logger = logging.getLogger(__name__)

class HealthStatus(str, Enum):
    """Health status values for scrapers"""
    HEALTHY = "healthy"
    DEGRADED = "degraded"
    FAILED = "failed"
    LIMITED = "limited"
    UNKNOWN = "unknown"


class HealthMonitor:
    """Base health monitoring for browser scrapers"""
    
    def __init__(self, source_type, source_name, source_detail):
        """Initialize the health monitor
        
        Args:
            source_type (str): Type of source (e.g. browser-scraper)
            source_name (str): Name identifier (e.g. slickcharts)
            source_detail (str): Human readable description
        """
        self.source_type = source_type
        self.source_name = source_name
        self.source_detail = source_detail
        self.db_conn = None
        
        # Connect to database
        try:
            self.db_conn = psycopg2.connect(
                host=os.environ.get("DB_HOST", "localhost"),
                port=os.environ.get("DB_PORT", "5432"),
                database=os.environ.get("DB_NAME", "quotron"),
                user=os.environ.get("DB_USER", "quotron"),
                password=os.environ.get("DB_PASSWORD", "quotron")
            )
            logger.info(f"Connected to database for health monitoring: {source_name}")
        except Exception as e:
            logger.error(f"Failed to connect to database: {e}")
    
    def update_health_status(self, status, error_message=None, response_time_ms=None, metadata=None):
        """Update the health status in the database
        
        Args:
            status (HealthStatus): Current health status
            error_message (str, optional): Error message if status is not healthy
            response_time_ms (int, optional): Response time in milliseconds
            metadata (dict, optional): Additional metadata to store
        """
        if self.db_conn is None:
            logger.error("Cannot update health status - no database connection")
            return
        
        try:
            # Convert metadata to JSON
            metadata_json = json.dumps(metadata) if metadata else None
            
            now = datetime.now()
            cursor = self.db_conn.cursor()
            
            # Check if the record exists
            cursor.execute(
                """
                SELECT id, error_count 
                FROM data_source_health 
                WHERE source_type = %s AND source_name = %s
                """,
                (self.source_type, self.source_name)
            )
            
            result = cursor.fetchone()
            
            if result:
                # Update existing record
                record_id, error_count = result
                if status in (HealthStatus.FAILED, "error"):
                    error_count += 1
                
                cursor.execute(
                    """
                    UPDATE data_source_health
                    SET status = %s, 
                        last_check = %s,
                        updated_at = %s,
                        error_message = %s,
                        response_time_ms = %s,
                        metadata = %s,
                        error_count = %s,
                        last_success = CASE 
                            WHEN %s IN ('healthy', 'degraded', 'limited') THEN %s
                            ELSE last_success 
                        END
                    WHERE id = %s
                    """,
                    (
                        status, now, now, error_message, response_time_ms, 
                        metadata_json, error_count, status, now, record_id
                    )
                )
            else:
                # Insert new record
                cursor.execute(
                    """
                    INSERT INTO data_source_health 
                    (source_type, source_name, source_detail, status, last_check, 
                     error_message, response_time_ms, metadata, created_at, updated_at)
                    VALUES (%s, %s, %s, %s, %s, %s, %s, %s, %s, %s)
                    """,
                    (
                        self.source_type, self.source_name, self.source_detail,
                        status, now, error_message, response_time_ms, 
                        metadata_json, now, now
                    )
                )
            
            self.db_conn.commit()
            logger.info(f"Updated health status for {self.source_name}: {status}")
            
        except Exception as e:
            logger.error(f"Error updating health status: {e}")
            try:
                self.db_conn.rollback()
            except:
                pass
    
    def close(self):
        """Close the database connection"""
        if self.db_conn:
            self.db_conn.close()
            self.db_conn = None


class SlickChartsHealthMonitor(HealthMonitor):
    """Health monitor for SlickCharts S&P 500 scraper"""
    
    def __init__(self):
        super().__init__(
            source_type="browser-scraper",
            source_name="slickcharts",
            source_detail="SlickCharts S&P 500 Scraper"
        )
        self.last_check_time = None
        self.success_count = 0
        self.error_count = 0
    
    def record_scrape_attempt(self, url, success, error_message=None, response_time_ms=None):
        """Record a scraping attempt
        
        Args:
            url (str): URL that was scraped
            success (bool): Whether the scrape was successful
            error_message (str, optional): Error message if failed
            response_time_ms (int, optional): Response time in milliseconds
        """
        now = time.time()
        self.last_check_time = now
        
        if success:
            self.success_count += 1
            status = HealthStatus.HEALTHY
        else:
            self.error_count += 1
            status = HealthStatus.FAILED
        
        metadata = {
            "url": url,
            "success_count": self.success_count,
            "error_count": self.error_count,
            "last_url": url
        }
        
        self.update_health_status(
            status=status,
            error_message=error_message,
            response_time_ms=response_time_ms,
            metadata=metadata
        )
        
        return status


class PlaywrightHealthMonitor(HealthMonitor):
    """Health monitor for generic Playwright scraper"""
    
    def __init__(self):
        super().__init__(
            source_type="browser-scraper",
            source_name="playwright",
            source_detail="Playwright Web Scraper"
        )
        self.pages_scraped = 0
        self.success_count = 0
        self.error_count = 0
        self.last_urls = []
    
    def record_scrape_attempt(self, url, success, error_message=None, response_time_ms=None):
        """Record a scraping attempt
        
        Args:
            url (str): URL that was scraped
            success (bool): Whether the scrape was successful
            error_message (str, optional): Error message if failed
            response_time_ms (int, optional): Response time in milliseconds
        """
        self.pages_scraped += 1
        
        # Keep track of last 5 URLs
        self.last_urls = [url] + self.last_urls[:4]
        
        if success:
            self.success_count += 1
            status = HealthStatus.HEALTHY
        else:
            self.error_count += 1
            status = HealthStatus.FAILED
        
        metadata = {
            "pages_scraped": self.pages_scraped,
            "success_count": self.success_count,
            "error_count": self.error_count,
            "success_rate": self.success_count / max(1, (self.success_count + self.error_count)),
            "last_urls": self.last_urls
        }
        
        self.update_health_status(
            status=status,
            error_message=error_message,
            response_time_ms=response_time_ms,
            metadata=metadata
        )
        
        return status