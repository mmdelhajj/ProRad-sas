#!/bin/bash
#
# Deploy SaaS Auto-Onboarding Update to SaaS Server
# Run this ON the SaaS server (139.162.153.201)
#
# Usage:
#   scp deploy-saas-package.tar.gz root@139.162.153.201:/tmp/
#   ssh root@139.162.153.201
#   cd /tmp && tar xzf deploy-saas-package.tar.gz && bash deploy-saas.sh
#

set -e

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

SAAS_DIR="/opt/proxpanel-saas"

echo -e "${GREEN}╔══════════════════════════════════════════════════════╗${NC}"
echo -e "${GREEN}║  SaaS Auto-Onboarding Deployment                    ║${NC}"
echo -e "${GREEN}╚══════════════════════════════════════════════════════╝${NC}"
echo ""

# Verify we're on the right server
if [ ! -d "$SAAS_DIR" ]; then
    echo -e "${RED}Error: $SAAS_DIR not found. Is this the SaaS server?${NC}"
    exit 1
fi

# Step 1: Backup current source
echo -e "${YELLOW}[1/6]${NC} Backing up current source..."
cp "$SAAS_DIR/backend/internal/radius/server.go" "$SAAS_DIR/backend/internal/radius/server.go.bak" 2>/dev/null || true
cp "$SAAS_DIR/backend/internal/handlers/onboarding.go" "$SAAS_DIR/backend/internal/handlers/onboarding.go.bak" 2>/dev/null || true
cp "$SAAS_DIR/backend/cmd/api/main.go" "$SAAS_DIR/backend/cmd/api/main.go.bak" 2>/dev/null || true
echo -e "    ${GREEN}Done${NC}"

# Step 2: Copy updated source files
echo -e "${YELLOW}[2/6]${NC} Copying updated source files..."
cp -v backend/internal/radius/server.go "$SAAS_DIR/backend/internal/radius/server.go"
cp -v backend/internal/handlers/onboarding.go "$SAAS_DIR/backend/internal/handlers/onboarding.go"
cp -v backend/cmd/api/main.go "$SAAS_DIR/backend/cmd/api/main.go"
echo -e "    ${GREEN}Done${NC}"

# Step 3: Copy updated frontend
echo -e "${YELLOW}[3/6]${NC} Copying updated frontend..."
cp -r frontend/dist/* "$SAAS_DIR/frontend/dist/"
cp -v frontend/src/services/api.js "$SAAS_DIR/frontend/src/services/api.js" 2>/dev/null || true
cp -v frontend/src/pages/Dashboard.jsx "$SAAS_DIR/frontend/src/pages/Dashboard.jsx" 2>/dev/null || true
echo -e "    ${GREEN}Done${NC}"

# Step 4: Update docker-compose with SAAS_RADIUS_SECRET
echo -e "${YELLOW}[4/6]${NC} Updating docker-compose.saas.yml..."
cp docker-compose.saas.yml "$SAAS_DIR/docker-compose.saas.yml"
echo -e "    ${GREEN}Done${NC}"

# Step 5: Rebuild Docker images
echo -e "${YELLOW}[5/6]${NC} Rebuilding Docker images (this may take 2-3 minutes)..."
cd "$SAAS_DIR"
docker compose -f docker-compose.saas.yml build api radius
echo -e "    ${GREEN}Done${NC}"

# Step 6: Restart containers
echo -e "${YELLOW}[6/6]${NC} Restarting containers..."
docker compose -f docker-compose.saas.yml up -d api radius
sleep 3
docker restart proxpanel-saas-frontend 2>/dev/null || true
echo -e "    ${GREEN}Done${NC}"

echo ""
echo -e "${GREEN}╔══════════════════════════════════════════════════════╗${NC}"
echo -e "${GREEN}║  Deployment Complete!                                ║${NC}"
echo -e "${GREEN}╚══════════════════════════════════════════════════════╝${NC}"
echo ""
echo -e "Verify:"
echo -e "  curl -s https://saas.proxrad.com/health | python3 -m json.tool"
echo -e "  docker logs proxpanel-saas-api --tail 20"
echo -e "  docker logs proxpanel-saas-radius --tail 20"
echo ""
echo -e "Test RADIUS status endpoint (login first, then):"
echo -e "  curl -s https://yahoo.saas.proxrad.com/api/radius-status -H 'Authorization: Bearer TOKEN'"
echo ""
