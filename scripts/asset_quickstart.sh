#!/bin/bash
# KD-Server Asset Management - Quick Start Script

set -e

GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
NC='\033[0m'

API_URL="${API_URL:-http://localhost:8080}"
TENANT="${KD_TENANT:-moe}"
API_KEY="${KD_API_KEY}"

if [ -z "$API_KEY" ]; then
    echo "Error: KD_API_KEY not set"
    exit 1
fi

echo -e "${BLUE}KD-Server Asset Management - Quick Start${NC}"
echo ""

api_call() {
    local method=$1
    local endpoint=$2
    local data=$3
    
    if [ -n "$data" ]; then
        curl -s -X $method "$API_URL$endpoint" \
            -H "Content-Type: application/json" \
            -H "Authorization: Bearer $API_KEY" \
            -H "X-KD-Tenant: $TENANT" \
            -d "$data"
    else
        curl -s -X $method "$API_URL$endpoint" \
            -H "Authorization: Bearer $API_KEY" \
            -H "X-KD-Tenant: $TENANT"
    fi
}

echo -e "${GREEN}[1/5] Fetching categories...${NC}"
CATEGORIES=$(api_call GET "/v1/assets/categories")
echo "   ✓ Found $(echo "$CATEGORIES" | jq '.categories | length') categories"

echo -e "${GREEN}[2/5] Creating test asset...${NC}"
INGEST=$(api_call POST "/v1/ingest/assets" '{"source":{"slug":"demo"},"records":[{"office":"B15","categoryNo":"02","typeNumber":"073","description1":"Test Table","location":"Lab","originalValue":5000,"acquisitionDate":"2024-11-28","isPurchasedNew":true}]}')
ASSET_NO=$(echo "$INGEST" | jq -r '.assetNumbers[0]')
echo "   ✓ Created: $ASSET_NO"

echo -e "${GREEN}[3/5] Looking up asset...${NC}"
ASSET=$(api_call GET "/v1/lookup/asset/$ASSET_NO")
echo "   ✓ Type: $(echo "$ASSET" | jq -r '.asset.typeName')"

echo -e "${GREEN}[4/5] Transferring asset...${NC}"
api_call POST "/v1/assets/$ASSET_NO/transfer" '{"toLocation":"New Lab","performedBy":"demo"}' > /dev/null
echo "   ✓ Transferred"

echo -e "${GREEN}[5/5] Checking history...${NC}"
HISTORY=$(api_call GET "/v1/assets/$ASSET_NO/history")
echo "$HISTORY" | jq -r '.history[] | "   • \(.eventType)"'

echo ""
echo -e "${BLUE}Demo Complete! Asset: $ASSET_NO${NC}"
