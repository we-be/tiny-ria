import urllib.request
import json
import sys

urls = [
    'http://localhost:5000/quote/AAPL',
    'http://localhost:8080/api/quote/AAPL'
]

for url in urls:
    try:
        print(f"\nTesting {url}:")
        response = urllib.request.urlopen(url)
        data = json.loads(response.read())
        print(json.dumps(data, indent=2))
    except Exception as e:
        print(f"Error accessing {url}: {e}", file=sys.stderr)