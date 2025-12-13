#!/bin/bash

# grx-fuzzer Demo Script
# This script demonstrates various fuzzing scenarios using grx-fuzzer

set -e

echo "================================================================"
echo "         grx-fuzzer - HTTP/HTTP2 Fuzzing Examples"
echo "================================================================"
echo ""

# Setup examples if not exists
if [ ! -d "./test/examples" ]; then
    echo "[*] Setting up example files..."
    bash ./setup-test.sh
    echo ""
fi

# Build grx-fuzzer if not exists
if [ ! -f "grx-fuzzer" ]; then
    echo "[*] Building grx-fuzzer..."
    go build -o grx-fuzzer .
    echo ""
fi

# Example 1: Simple HTTP/1.1 Path Fuzzing
echo "====== Example 1: HTTP/1.1 Path Fuzzing ======"
echo "Target: http://httpbin.org/PATH"
echo ""

./grx-fuzzer --host httpbin.org \
  --request $'GET /PATH HTTP/1.1\r\nHost: httpbin.org\r\nUser-Agent: grx-fuzzer\r\n\r\n' \
  --marker "PATH=test/examples/paths.txt" \
  --concurrency 5 \
  --output test/results-http1-paths.json

echo ""
echo "Press Enter to continue..."
read

# Example 2: HTTP/2 Fuzzing
echo ""
echo "====== Example 2: HTTP/2 Fuzzing ======"
echo "Target: https://www.google.com/"
echo ""

# Create single.txt BEFORE using it
echo "test" > test/examples/single.txt

./grx-fuzzer --host www.google.com --tls --http2 \
  --request $'GET / HTTP/1.1\r\nHost: www.google.com\r\nUser-Agent: grx-fuzzer\r\n\r\n' \
  --marker "DUMMY=test/examples/single.txt" \
  --concurrency 1 \
  --output test/results-http2-google.json \
  --verbose

echo ""
echo "Press Enter to continue..."
read

# Example 3: API Endpoint Discovery (HTTP/2)
echo ""
echo "====== Example 3: API Endpoint Discovery ======"
echo "Target: https://httpbin.org/ENDPOINT"
echo ""

./grx-fuzzer --host httpbin.org --tls \
  --request $'GET /ENDPOINT HTTP/1.1\r\nHost: httpbin.org\r\nAccept: application/json\r\n\r\n' \
  --marker "ENDPOINT=test/examples/api-endpoints.txt" \
  --concurrency 10 \
  --output test/results-api-discovery.json

echo ""
echo "Press Enter to continue..."
read

# Example 4: Using Request File
echo ""
echo "====== Example 4: Using Request File ======"
echo "Target: https://httpbin.org/anything/ENDPOINT"
echo ""

./grx-fuzzer --host httpbin.org --tls \
  --request-file test/examples/request-get.txt \
  --marker "ENDPOINT=test/examples/api-endpoints.txt" \
  --concurrency 10 \
  --output test/results-request-file.json

echo ""
echo "Press Enter to continue..."
read

# Example 5: Verbose Mode (shows all results)
echo ""
echo "====== Example 5: Verbose Mode ======"
echo "Target: https://httpbin.org/status/CODE"
echo ""

# Create status codes wordlist
cat > test/examples/status-codes.txt << 'EOF'
200
201
204
301
302
400
401
403
404
500
EOF

./grx-fuzzer --host httpbin.org --tls \
  --request $'GET /status/CODE HTTP/1.1\r\nHost: httpbin.org\r\n\r\n' \
  --marker "CODE=test/examples/status-codes.txt" \
  --concurrency 5 \
  --verbose \
  --output test/results-verbose.json

echo ""
echo "================================================================"
echo "                     Demo Complete!"
echo "================================================================"
echo ""
echo "Results saved to:"
echo "  - results-http1-paths.json"
echo "  - results-http2-google.json"
echo "  - results-api-discovery.json"
echo "  - results-request-file.json"
echo "  - results-verbose.json"
echo ""
echo "View results: cat results-*.json | jq"
