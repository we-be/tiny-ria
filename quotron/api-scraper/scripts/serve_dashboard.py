#!/usr/bin/env python3
"""
Simple HTTP server to serve the economic dashboard
"""

import http.server
import socketserver
import os
import sys

PORT = 8008

# Simple HTTP server that also handles CORS for the dashboard
class Handler(http.server.SimpleHTTPRequestHandler):
    """Custom handler for serving files"""
    
    def end_headers(self):
        """Add CORS headers before ending headers"""
        self.send_header('Access-Control-Allow-Origin', '*')
        self.send_header('Access-Control-Allow-Methods', 'GET, OPTIONS')
        self.send_header('Access-Control-Allow-Headers', 'Content-Type')
        super().end_headers()
    
    def do_OPTIONS(self):
        """Handle OPTIONS requests for CORS preflight"""
        self.send_response(204)  # No content
        self.end_headers()
    
    def do_GET(self):
        """Handle GET requests"""
        if self.path == '/' or self.path == '/index.html':
            # Serve the dashboard HTML
            self.send_response(200)
            self.send_header('Content-type', 'text/html')
            self.end_headers()
            
            with open('economic_dashboard.html', 'rb') as file:
                self.wfile.write(file.read())
        else:
            # Let the parent class handle other requests
            super().do_GET()

def main():
    """Run the server"""
    # Change to the directory containing this script
    script_dir = os.path.dirname(os.path.abspath(__file__))
    os.chdir(script_dir)
    
    # Create the server with the custom handler
    handler = Handler
    
    # Enable directory listing for static files
    handler.extensions_map.update({
        '': 'application/octet-stream',
        '.html': 'text/html',
        '.js': 'application/javascript',
        '.css': 'text/css',
    })
    
    print(f"Starting server on port {PORT}...")
    with socketserver.TCPServer(("", PORT), handler) as httpd:
        print(f"Server running at http://localhost:{PORT}")
        print("Press Ctrl+C to stop")
        try:
            httpd.serve_forever()
        except KeyboardInterrupt:
            print("\nServer stopped")

if __name__ == "__main__":
    main()