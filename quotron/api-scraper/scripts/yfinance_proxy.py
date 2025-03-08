#!/usr/bin/env python3
"""
Yahoo Finance Proxy Server

This script creates a small HTTP server that serves as a proxy for Yahoo Finance data.
It uses the yfinance Python package to fetch financial data and serves it via a REST API.
"""

import argparse
import json
import logging
from datetime import datetime
from http.server import BaseHTTPRequestHandler, HTTPServer
from urllib.parse import parse_qs, urlparse

import yfinance as yf

# Configure logging
logging.basicConfig(
    level=logging.INFO,
    format="%(asctime)s - %(name)s - %(levelname)s - %(message)s",
)
logger = logging.getLogger("yfinance-proxy")

class YahooFinanceProxy(BaseHTTPRequestHandler):
    """HTTP request handler for the Yahoo Finance proxy."""

    def _set_headers(self, content_type="application/json"):
        self.send_response(200)
        self.send_header("Content-type", content_type)
        self.send_header("Access-Control-Allow-Origin", "*")
        self.end_headers()

    def _handle_error(self, status_code, message):
        self.send_response(status_code)
        self.send_header("Content-type", "application/json")
        self.end_headers()
        self.wfile.write(json.dumps({"error": message}).encode())

    def get_stock_quote(self, symbol):
        """Fetch stock quote data for a symbol."""
        try:
            ticker = yf.Ticker(symbol)
            info = ticker.info
            
            # Create a simplified quote object similar to our Go model
            quote = {
                "symbol": symbol,
                "price": info.get("regularMarketPrice", 0.0),
                "change": info.get("regularMarketChange", 0.0),
                "changePercent": info.get("regularMarketChangePercent", 0.0),
                "volume": info.get("regularMarketVolume", 0),
                "timestamp": datetime.now().strftime("%Y-%m-%dT%H:%M:%SZ"),
                "exchange": info.get("exchange", ""),
                "source": "Yahoo Finance (Python)",
            }
            return quote
            
        except Exception as e:
            logger.error(f"Error fetching quote for {symbol}: {e}")
            return {"error": str(e), "symbol": symbol}

    def get_market_data(self, index):
        """Fetch market index data."""
        try:
            ticker = yf.Ticker(index)
            info = ticker.info
            
            # Create a simplified market data object similar to our Go model
            market_data = {
                "indexName": info.get("shortName", index),
                "value": info.get("regularMarketPrice", 0.0),
                "change": info.get("regularMarketChange", 0.0),
                "changePercent": info.get("regularMarketChangePercent", 0.0),
                "timestamp": datetime.now().strftime("%Y-%m-%dT%H:%M:%SZ"),
                "source": "Yahoo Finance (Python)",
            }
            return market_data
            
        except Exception as e:
            logger.error(f"Error fetching market data for {index}: {e}")
            return {"error": str(e), "index": index}

    def do_GET(self):
        """Handle GET requests."""
        parsed_url = urlparse(self.path)
        path = parsed_url.path
        query_params = parse_qs(parsed_url.query)
        
        # Route: /quote?symbol=AAPL
        if path == "/quote":
            if "symbol" not in query_params:
                self._handle_error(400, "Missing symbol parameter")
                return
                
            symbol = query_params["symbol"][0]
            quote = self.get_stock_quote(symbol)
            
            self._set_headers()
            self.wfile.write(json.dumps(quote).encode())
            
        # Route: /market?index=^GSPC
        elif path == "/market":
            if "index" not in query_params:
                self._handle_error(400, "Missing index parameter")
                return
                
            index = query_params["index"][0]
            market_data = self.get_market_data(index)
            
            self._set_headers()
            self.wfile.write(json.dumps(market_data).encode())
            
        # Route: /health
        elif path == "/health":
            self._set_headers()
            self.wfile.write(json.dumps({"status": "ok"}).encode())
            
        else:
            self._handle_error(404, f"Endpoint not found: {path}")

def run_server(host="localhost", port=8080):
    """Run the HTTP server."""
    server = HTTPServer((host, port), YahooFinanceProxy)
    logger.info(f"Starting Yahoo Finance proxy server on http://{host}:{port}")
    logger.info("Press Ctrl+C to stop the server")
    
    try:
        server.serve_forever()
    except KeyboardInterrupt:
        logger.info("Server stopped by user")
    
    server.server_close()
    logger.info("Server stopped")

if __name__ == "__main__":
    parser = argparse.ArgumentParser(description="Yahoo Finance Proxy Server")
    parser.add_argument("--host", default="localhost", help="Host to bind the server to")
    parser.add_argument("--port", type=int, default=8080, help="Port to bind the server to")
    args = parser.parse_args()
    
    run_server(args.host, args.port)