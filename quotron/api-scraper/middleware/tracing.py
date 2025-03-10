import time
import uuid
import json
import psycopg2
import psycopg2.extras
from functools import wraps
from flask import request, g, current_app
import os

class TracingMiddleware:
    """Middleware for tracing Flask requests."""
    
    def __init__(self, app=None, service_name=None, db_params=None):
        self.app = app
        self.service_name = service_name or "yahoo_finance_proxy"
        self.db_params = db_params or {
            "host": os.environ.get("DB_HOST", "localhost"),
            "port": os.environ.get("DB_PORT", "5432"),
            "dbname": os.environ.get("DB_NAME", "quotron"),
            "user": os.environ.get("DB_USER", "quotron"),
            "password": os.environ.get("DB_PASSWORD", "quotron")
        }
        
        if app is not None:
            self.init_app(app)
    
    def init_app(self, app):
        """Initialize the middleware with a Flask app."""
        app.before_request(self.before_request)
        app.after_request(self.after_request)
        app.teardown_request(self.teardown_request)
    
    def connect_db(self):
        """Connect to the database."""
        try:
            return psycopg2.connect(**self.db_params)
        except Exception as e:
            current_app.logger.error(f"Error connecting to database: {e}")
            return None
    
    def before_request(self):
        """Process the request before it's handled by the route function."""
        # Generate trace ID or use from header if it exists
        trace_id = request.headers.get('X-Trace-ID')
        if not trace_id:
            trace_id = str(uuid.uuid4())
        
        # Get parent span ID if it exists
        parent_span_id = request.headers.get('X-Span-ID')
        
        # Generate span ID for this request
        span_id = str(uuid.uuid4())
        
        # Store the trace information in Flask's g object
        g.trace_id = trace_id
        g.parent_span_id = parent_span_id
        g.span_id = span_id
        g.start_time = time.time()
        
        # Log the trace start
        current_app.logger.debug(f"Trace started: {trace_id}, span: {span_id}")
    
    def after_request(self, response):
        """Process the response before it's sent to the client."""
        # Add trace headers to response
        response.headers['X-Trace-ID'] = getattr(g, 'trace_id', str(uuid.uuid4()))
        response.headers['X-Span-ID'] = getattr(g, 'span_id', str(uuid.uuid4()))
        
        return response
    
    def teardown_request(self, exception=None):
        """Store the trace information in the database."""
        if not hasattr(g, 'start_time'):
            return
        
        # Calculate duration
        end_time = time.time()
        duration_ms = int((end_time - g.start_time) * 1000)
        
        # Determine the name of the operation (endpoint)
        name = request.path
        
        # Determine status based on exception
        status = "success"
        error_message = None
        if exception:
            status = "error"
            error_message = str(exception)
        elif hasattr(g, 'response_status_code') and g.response_status_code >= 400:
            status = "error"
            error_message = f"HTTP {g.response_status_code}"
        
        # Create metadata
        metadata = {
            "http_method": request.method,
            "http_path": request.path,
            "user_agent": request.user_agent.string if request.user_agent else None,
        }
        
        # Add query parameters if any, but remove sensitive data
        if request.args:
            queries = {}
            for k, v in request.args.items():
                # Skip sensitive parameters
                if k in ["api_key", "key", "token", "password"]:
                    queries[k] = "[REDACTED]"
                else:
                    queries[k] = v
            metadata["query_params"] = queries
        
        # Convert metadata to JSON
        metadata_json = json.dumps(metadata)
        
        # Store the trace in the database
        try:
            conn = self.connect_db()
            if conn:
                with conn.cursor() as cur:
                    cur.execute(
                        """INSERT INTO service_traces 
                        (trace_id, parent_id, name, service, start_time, end_time, duration_ms, status, error_message, metadata) 
                        VALUES (%s, %s, %s, %s, %s, %s, %s, %s, %s, %s)""",
                        (
                            g.trace_id, 
                            g.parent_span_id,
                            name,
                            self.service_name,
                            time.strftime('%Y-%m-%d %H:%M:%S.%f', time.gmtime(g.start_time)),
                            time.strftime('%Y-%m-%d %H:%M:%S.%f', time.gmtime(end_time)),
                            duration_ms,
                            status,
                            error_message,
                            metadata_json
                        )
                    )
                conn.commit()
                conn.close()
        except Exception as e:
            current_app.logger.error(f"Error storing trace: {e}")


# Decorator for tracing specific functions
def trace_function(func=None, name=None, service=None):
    """Decorator to trace function calls."""
    def decorator(f):
        @wraps(f)
        def wrapped(*args, **kwargs):
            # Get trace ID from g if available, otherwise generate new one
            trace_id = getattr(g, 'trace_id', str(uuid.uuid4()))
            parent_span_id = getattr(g, 'span_id', None)
            span_id = str(uuid.uuid4())
            
            # Set span ID for this function
            g.current_func_span_id = span_id
            
            # Determine function name
            func_name = name or f.__name__
            service_name = service or "function"
            
            # Record start time
            start_time = time.time()
            
            # Execute the function
            try:
                result = f(*args, **kwargs)
                status = "success"
                error_message = None
                return result
            except Exception as e:
                status = "error"
                error_message = str(e)
                raise
            finally:
                # Calculate duration
                end_time = time.time()
                duration_ms = int((end_time - start_time) * 1000)
                
                # Create metadata
                metadata = {"function": func_name}
                if kwargs:
                    # Filter out sensitive parameters
                    filtered_kwargs = {}
                    for k, v in kwargs.items():
                        if k in ["api_key", "key", "token", "password"]:
                            filtered_kwargs[k] = "[REDACTED]"
                        else:
                            filtered_kwargs[k] = str(v)
                    metadata["kwargs"] = filtered_kwargs
                
                # Convert metadata to JSON
                metadata_json = json.dumps(metadata)
                
                # Store the trace in the database if we have current_app
                if current_app:
                    try:
                        # Get database parameters from app config
                        db_params = {
                            "host": current_app.config.get("DB_HOST", os.environ.get("DB_HOST", "localhost")),
                            "port": current_app.config.get("DB_PORT", os.environ.get("DB_PORT", "5432")),
                            "dbname": current_app.config.get("DB_NAME", os.environ.get("DB_NAME", "quotron")),
                            "user": current_app.config.get("DB_USER", os.environ.get("DB_USER", "quotron")),
                            "password": current_app.config.get("DB_PASSWORD", os.environ.get("DB_PASSWORD", "quotron"))
                        }
                        
                        conn = psycopg2.connect(**db_params)
                        with conn.cursor() as cur:
                            cur.execute(
                                """INSERT INTO service_traces 
                                (trace_id, parent_id, name, service, start_time, end_time, duration_ms, status, error_message, metadata) 
                                VALUES (%s, %s, %s, %s, %s, %s, %s, %s, %s, %s)""",
                                (
                                    trace_id, 
                                    parent_span_id,
                                    func_name,
                                    service_name,
                                    time.strftime('%Y-%m-%d %H:%M:%S.%f', time.gmtime(start_time)),
                                    time.strftime('%Y-%m-%d %H:%M:%S.%f', time.gmtime(end_time)),
                                    duration_ms,
                                    status,
                                    error_message,
                                    metadata_json
                                )
                            )
                        conn.commit()
                        conn.close()
                    except Exception as e:
                        if current_app:
                            current_app.logger.error(f"Error storing function trace: {e}")
                
                # Reset the current function span ID
                g.current_func_span_id = parent_span_id
        
        return wrapped
    
    if func:
        return decorator(func)
    return decorator