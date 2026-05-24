#!/bin/bash

# Eagle Bank E2E Test Script
# This script tests all API endpoints with sample data

set -e

BASE_URL="${BASE_URL:-http://localhost:8080}"
CONTENT_TYPE="Content-Type: application/json"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Store tokens and IDs
ACCESS_TOKEN=""
USER_ID=""
ACCOUNT_NUMBER=""
TRANSACTION_ID=""

print_header() {
    echo ""
    echo -e "${BLUE}═══════════════════════════════════════════════════════════════${NC}"
    echo -e "${BLUE}  $1${NC}"
    echo -e "${BLUE}═══════════════════════════════════════════════════════════════${NC}"
}

print_step() {
    echo ""
    echo -e "${YELLOW}▶ $1${NC}"
}

print_success() {
    echo -e "${GREEN}✓ $1${NC}"
}

print_error() {
    echo -e "${RED}✗ $1${NC}"
}

print_response() {
    echo "$1" | jq '.' 2>/dev/null || echo "$1"
}

check_status() {
    local expected=$1
    local actual=$2
    local description=$3

    if [ "$actual" -eq "$expected" ]; then
        print_success "$description (HTTP $actual)"
        return 0
    else
        print_error "$description (Expected HTTP $expected, got HTTP $actual)"
        return 1
    fi
}

# ═══════════════════════════════════════════════════════════════
# HEALTH CHECKS
# ═══════════════════════════════════════════════════════════════
print_header "HEALTH CHECKS"

print_step "Testing /health endpoint"
RESPONSE=$(curl -s -w "\n%{http_code}" "$BASE_URL/health")
HTTP_CODE=$(echo "$RESPONSE" | tail -n 1)
BODY=$(echo "$RESPONSE" | sed '$d')
check_status 200 "$HTTP_CODE" "Health check"
print_response "$BODY"

print_step "Testing /ready endpoint"
RESPONSE=$(curl -s -w "\n%{http_code}" "$BASE_URL/ready")
HTTP_CODE=$(echo "$RESPONSE" | tail -n 1)
BODY=$(echo "$RESPONSE" | sed '$d')
check_status 200 "$HTTP_CODE" "Readiness check"
print_response "$BODY"

print_step "Testing /live endpoint"
RESPONSE=$(curl -s -w "\n%{http_code}" "$BASE_URL/live")
HTTP_CODE=$(echo "$RESPONSE" | tail -n 1)
BODY=$(echo "$RESPONSE" | sed '$d')
check_status 200 "$HTTP_CODE" "Liveness check"
print_response "$BODY"

print_step "Testing /metrics endpoint"
RESPONSE=$(curl -s -w "\n%{http_code}" "$BASE_URL/metrics")
HTTP_CODE=$(echo "$RESPONSE" | tail -n 1)
check_status 200 "$HTTP_CODE" "Metrics endpoint"
echo "Metrics output (truncated):"
echo "$RESPONSE" | sed '$d' | head -20
echo "..."

# ═══════════════════════════════════════════════════════════════
# USER REGISTRATION
# ═══════════════════════════════════════════════════════════════
print_header "USER REGISTRATION"

print_step "Creating a new user"
USER_EMAIL="john.doe.$(date +%s)@example.com"
RESPONSE=$(curl -s -w "\n%{http_code}" -X POST "$BASE_URL/v1/users" \
    -H "$CONTENT_TYPE" \
    -d '{
        "name": "John Doe",
        "email": "'"$USER_EMAIL"'",
        "password": "SecurePassword123!",
        "phoneNumber": "+1234567890",
        "address": {
            "line1": "123 Main Street",
            "town": "London",
            "county": "Greater London",
            "postcode": "SW1A 1AA"
        }
    }')

HTTP_CODE=$(echo "$RESPONSE" | tail -n 1)
BODY=$(echo "$RESPONSE" | sed '$d')
check_status 201 "$HTTP_CODE" "User registration"
print_response "$BODY"

# Extract user ID
USER_ID=$(echo "$BODY" | jq -r '.id // .userId // empty')
if [ -n "$USER_ID" ]; then
    print_success "User ID: $USER_ID"
else
    print_error "Failed to extract user ID"
fi

# ═══════════════════════════════════════════════════════════════
# USER LOGIN
# ═══════════════════════════════════════════════════════════════
print_header "USER LOGIN"

print_step "Logging in with created user"
RESPONSE=$(curl -s -w "\n%{http_code}" -X POST "$BASE_URL/v1/login" \
    -H "$CONTENT_TYPE" \
    -d '{
        "email": "'"$USER_EMAIL"'",
        "password": "SecurePassword123!"
    }')

HTTP_CODE=$(echo "$RESPONSE" | tail -n 1)
BODY=$(echo "$RESPONSE" | sed '$d')
check_status 200 "$HTTP_CODE" "User login"
print_response "$BODY"

# Extract access token
ACCESS_TOKEN=$(echo "$BODY" | jq -r '.accessToken // empty')
if [ -n "$ACCESS_TOKEN" ]; then
    print_success "Access token obtained (truncated): ${ACCESS_TOKEN:0:50}..."
else
    print_error "Failed to obtain access token"
    exit 1
fi

AUTH_HEADER="Authorization: Bearer $ACCESS_TOKEN"

# ═══════════════════════════════════════════════════════════════
# USER OPERATIONS
# ═══════════════════════════════════════════════════════════════
print_header "USER OPERATIONS"

print_step "Getting user profile"
RESPONSE=$(curl -s -w "\n%{http_code}" -X GET "$BASE_URL/v1/users/$USER_ID" \
    -H "$AUTH_HEADER")

HTTP_CODE=$(echo "$RESPONSE" | tail -n 1)
BODY=$(echo "$RESPONSE" | sed '$d')
check_status 200 "$HTTP_CODE" "Get user profile"
print_response "$BODY"

print_step "Updating user profile"
RESPONSE=$(curl -s -w "\n%{http_code}" -X PATCH "$BASE_URL/v1/users/$USER_ID" \
    -H "$AUTH_HEADER" \
    -H "$CONTENT_TYPE" \
    -d '{
        "name": "Jonathan Doe",
        "phoneNumber": "+1987654321"
    }')

HTTP_CODE=$(echo "$RESPONSE" | tail -n 1)
BODY=$(echo "$RESPONSE" | sed '$d')
check_status 200 "$HTTP_CODE" "Update user profile"
print_response "$BODY"

# ═══════════════════════════════════════════════════════════════
# ACCOUNT OPERATIONS
# ═══════════════════════════════════════════════════════════════
print_header "ACCOUNT OPERATIONS"

print_step "Creating a personal account"
RESPONSE=$(curl -s -w "\n%{http_code}" -X POST "$BASE_URL/v1/accounts" \
    -H "$AUTH_HEADER" \
    -H "$CONTENT_TYPE" \
    -d '{
        "name": "My Current Account",
        "accountType": "personal"
    }')

HTTP_CODE=$(echo "$RESPONSE" | tail -n 1)
BODY=$(echo "$RESPONSE" | sed '$d')
check_status 201 "$HTTP_CODE" "Create personal account"
print_response "$BODY"

# Extract account number
ACCOUNT_NUMBER=$(echo "$BODY" | jq -r '.accountNumber // empty')
if [ -n "$ACCOUNT_NUMBER" ]; then
    print_success "Account Number: $ACCOUNT_NUMBER"
else
    print_error "Failed to extract account number"
    exit 1
fi

print_step "Creating a second personal account"
RESPONSE=$(curl -s -w "\n%{http_code}" -X POST "$BASE_URL/v1/accounts" \
    -H "$AUTH_HEADER" \
    -H "$CONTENT_TYPE" \
    -d '{
        "name": "My Savings Account",
        "accountType": "personal"
    }')

HTTP_CODE=$(echo "$RESPONSE" | tail -n 1)
BODY=$(echo "$RESPONSE" | sed '$d')
check_status 201 "$HTTP_CODE" "Create second personal account"
print_response "$BODY"

print_step "Listing all accounts"
RESPONSE=$(curl -s -w "\n%{http_code}" -X GET "$BASE_URL/v1/accounts" \
    -H "$AUTH_HEADER")

HTTP_CODE=$(echo "$RESPONSE" | tail -n 1)
BODY=$(echo "$RESPONSE" | sed '$d')
check_status 200 "$HTTP_CODE" "List accounts"
print_response "$BODY"

print_step "Getting single account"
RESPONSE=$(curl -s -w "\n%{http_code}" -X GET "$BASE_URL/v1/accounts/$ACCOUNT_NUMBER" \
    -H "$AUTH_HEADER")

HTTP_CODE=$(echo "$RESPONSE" | tail -n 1)
BODY=$(echo "$RESPONSE" | sed '$d')
check_status 200 "$HTTP_CODE" "Get single account"
print_response "$BODY"

# ═══════════════════════════════════════════════════════════════
# TRANSACTION OPERATIONS
# ═══════════════════════════════════════════════════════════════
print_header "TRANSACTION OPERATIONS"

print_step "Making a deposit of £1000"
RESPONSE=$(curl -s -w "\n%{http_code}" -X POST "$BASE_URL/v1/accounts/$ACCOUNT_NUMBER/transactions" \
    -H "$AUTH_HEADER" \
    -H "$CONTENT_TYPE" \
    -d '{
        "type": "deposit",
        "amount": 1000.00,
        "currency": "GBP",
        "reference": "Initial deposit"
    }')

HTTP_CODE=$(echo "$RESPONSE" | tail -n 1)
BODY=$(echo "$RESPONSE" | sed '$d')
check_status 201 "$HTTP_CODE" "Deposit transaction"
print_response "$BODY"

TRANSACTION_ID=$(echo "$BODY" | jq -r '.id // .transactionId // empty')
if [ -n "$TRANSACTION_ID" ]; then
    print_success "Transaction ID: $TRANSACTION_ID"
fi

print_step "Making another deposit of £500"
RESPONSE=$(curl -s -w "\n%{http_code}" -X POST "$BASE_URL/v1/accounts/$ACCOUNT_NUMBER/transactions" \
    -H "$AUTH_HEADER" \
    -H "$CONTENT_TYPE" \
    -d '{
        "type": "deposit",
        "amount": 500.00,
        "currency": "GBP",
        "reference": "Salary payment"
    }')

HTTP_CODE=$(echo "$RESPONSE" | tail -n 1)
BODY=$(echo "$RESPONSE" | sed '$d')
check_status 201 "$HTTP_CODE" "Second deposit"
print_response "$BODY"

print_step "Making a withdrawal of £250"
RESPONSE=$(curl -s -w "\n%{http_code}" -X POST "$BASE_URL/v1/accounts/$ACCOUNT_NUMBER/transactions" \
    -H "$AUTH_HEADER" \
    -H "$CONTENT_TYPE" \
    -d '{
        "type": "withdrawal",
        "amount": 250.00,
        "currency": "GBP",
        "reference": "ATM withdrawal"
    }')

HTTP_CODE=$(echo "$RESPONSE" | tail -n 1)
BODY=$(echo "$RESPONSE" | sed '$d')
check_status 201 "$HTTP_CODE" "Withdrawal transaction"
print_response "$BODY"

print_step "Listing all transactions"
RESPONSE=$(curl -s -w "\n%{http_code}" -X GET "$BASE_URL/v1/accounts/$ACCOUNT_NUMBER/transactions" \
    -H "$AUTH_HEADER")

HTTP_CODE=$(echo "$RESPONSE" | tail -n 1)
BODY=$(echo "$RESPONSE" | sed '$d')
check_status 200 "$HTTP_CODE" "List transactions"
print_response "$BODY"

if [ -n "$TRANSACTION_ID" ]; then
    print_step "Getting single transaction"
    RESPONSE=$(curl -s -w "\n%{http_code}" -X GET "$BASE_URL/v1/accounts/$ACCOUNT_NUMBER/transactions/$TRANSACTION_ID" \
        -H "$AUTH_HEADER")

    HTTP_CODE=$(echo "$RESPONSE" | tail -n 1)
    BODY=$(echo "$RESPONSE" | sed '$d')
    check_status 200 "$HTTP_CODE" "Get single transaction"
    print_response "$BODY"
fi

print_step "Checking account balance after transactions"
RESPONSE=$(curl -s -w "\n%{http_code}" -X GET "$BASE_URL/v1/accounts/$ACCOUNT_NUMBER" \
    -H "$AUTH_HEADER")

HTTP_CODE=$(echo "$RESPONSE" | tail -n 1)
BODY=$(echo "$RESPONSE" | sed '$d')
check_status 200 "$HTTP_CODE" "Get account balance"
print_response "$BODY"

BALANCE=$(echo "$BODY" | jq -r '.balance // empty')
print_success "Current balance: £$BALANCE (expected: £1250.00)"

# ═══════════════════════════════════════════════════════════════
# ERROR HANDLING TESTS
# ═══════════════════════════════════════════════════════════════
print_header "ERROR HANDLING TESTS"

print_step "Testing insufficient funds (withdrawal of \$10000)"
RESPONSE=$(curl -s -w "\n%{http_code}" -X POST "$BASE_URL/v1/accounts/$ACCOUNT_NUMBER/transactions" \
    -H "$AUTH_HEADER" \
    -H "$CONTENT_TYPE" \
    -d '{
        "type": "withdrawal",
        "amount": 10000.00,
        "currency": "GBP",
        "reference": "Should fail"
    }')

HTTP_CODE=$(echo "$RESPONSE" | tail -n 1)
BODY=$(echo "$RESPONSE" | sed '$d')
if [ "$HTTP_CODE" -ge 400 ]; then
    print_success "Insufficient funds correctly rejected (HTTP $HTTP_CODE)"
else
    print_error "Should have rejected insufficient funds"
fi
print_response "$BODY"

print_step "Testing invalid account number"
RESPONSE=$(curl -s -w "\n%{http_code}" -X GET "$BASE_URL/v1/accounts/invalid123" \
    -H "$AUTH_HEADER")

HTTP_CODE=$(echo "$RESPONSE" | tail -n 1)
BODY=$(echo "$RESPONSE" | sed '$d')
if [ "$HTTP_CODE" -ge 400 ]; then
    print_success "Invalid account correctly rejected (HTTP $HTTP_CODE)"
else
    print_error "Should have rejected invalid account"
fi
print_response "$BODY"

print_step "Testing unauthorized access (no token)"
RESPONSE=$(curl -s -w "\n%{http_code}" -X GET "$BASE_URL/v1/accounts")

HTTP_CODE=$(echo "$RESPONSE" | tail -n 1)
BODY=$(echo "$RESPONSE" | sed '$d')
if [ "$HTTP_CODE" -eq 401 ]; then
    print_success "Unauthorized correctly rejected (HTTP $HTTP_CODE)"
else
    print_error "Should have rejected unauthorized request"
fi
print_response "$BODY"

# ═══════════════════════════════════════════════════════════════
# SUMMARY
# ═══════════════════════════════════════════════════════════════
print_header "TEST SUMMARY"

echo ""
echo -e "${GREEN}Test Data Created:${NC}"
echo "  User Email:      $USER_EMAIL"
echo "  User ID:         $USER_ID"
echo "  Account Number:  $ACCOUNT_NUMBER"
echo "  Final Balance:   \$$BALANCE"
echo ""
echo -e "${BLUE}UI Access Points:${NC}"
echo "  API:        http://localhost:8080"
echo "  Kafka UI:   http://localhost:8090"
echo "  pgAdmin:    http://localhost:5050 (admin@eagle-bank.io / admin123)"
echo "  Jaeger:     http://localhost:16686"
echo "  Prometheus: http://localhost:9090"
echo "  Grafana:    http://localhost:3000 (admin / admin)"
echo ""
echo -e "${GREEN}All tests completed!${NC}"
