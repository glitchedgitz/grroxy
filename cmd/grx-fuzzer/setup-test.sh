#!/bin/bash

# Example wordlists and request files for grx-fuzzer

# Create examples directory
mkdir -p test
mkdir -p test/examples

# Example 1: Create a simple paths wordlist
cat > test/examples/paths.txt << 'EOF'
admin
api
users
login
config
dashboard
settings
profile
EOF

# Example 2: Create a usernames wordlist
cat > test/examples/usernames.txt << 'EOF'
admin
user
test
root
guest
EOF

# Example 3: Create a passwords wordlist  
cat > test/examples/passwords.txt << 'EOF'
password
admin123
test123
root123
guest123
EOF

# Example 4: Create an API endpoints wordlist
cat > test/examples/api-endpoints.txt << 'EOF'
users
posts
comments
products
orders
customers
stats
health
EOF

# Example 5: Create tokens wordlist
cat > test/examples/tokens.txt << 'EOF'
token1
token2
token3
invalid-token
expired-token
EOF

# Example 6: Create a complete HTTP request file
cat > test/examples/request-get.txt << 'EOF'
GET /api/ENDPOINT HTTP/1.1
Host: api.example.com
User-Agent: grx-fuzzer
Accept: application/json
Authorization: Bearer test-token

EOF

# Example 7: Create a POST request file
cat > test/examples/request-post-login.txt << 'EOF'
POST /api/login HTTP/1.1
Host: auth.example.com
Content-Type: application/json
User-Agent: grx-fuzzer

{"username":"USER","password":"PASS"}
EOF

echo "โœ… Example files created in ./test/examples/"
echo ""
echo "Available files:"
ls -lh test/examples/
