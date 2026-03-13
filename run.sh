#!/bin/bash

# ==============================================================================
# Solana USDC Paywall - All-in-One Setup & Run Script
# ==============================================================================

# Colors for output
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m' # No Color

echo -e "${BLUE}======================================================${NC}"
echo -e "${BLUE}   🚀 Starting Solana USDC Paywall Setup & Run   ${NC}"
echo -e "${BLUE}======================================================${NC}"

# 1. Prerequisite Checks
# ------------------------------------------------------------------------------
echo -e "${YELLOW}[1/5] Checking prerequisites...${NC}"

if ! command -v go &> /dev/null; then
    echo -e "${RED}Error: Go is not installed. Please install Go (Golang).${NC}"
    exit 1
fi

if ! command -v npm &> /dev/null; then
    echo -e "${RED}Error: npm is not installed. Please install Node.js.${NC}"
    exit 1
fi

# Check for migrate tool (required by Makefile)
if ! command -v migrate &> /dev/null; then
    echo -e "${YELLOW}Warning: 'migrate' tool not found. Attempting to install via Go...${NC}"
    go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest
    # Add go bin to path for this session if not present
    export PATH=$PATH:$(go env GOPATH)/bin
    
    if ! command -v migrate &> /dev/null; then
         echo -e "${RED}Error: Failed to install 'migrate'. Please install it manually.${NC}"
         echo -e "See: https://github.com/golang-migrate/migrate/tree/master/cmd/migrate"
         exit 1
    fi
    echo -e "${GREEN}✓ 'migrate' tool installed.${NC}"
fi

echo -e "${GREEN}✓ Prerequisites look good.${NC}"

# 2. Backend Setup
# ------------------------------------------------------------------------------
echo -e "\n${YELLOW}[2/5] Setting up Backend...${NC}"
cd backend || { echo -e "${RED}Error: 'backend' directory not found.${NC}"; exit 1; }

# Handle .env
if [ ! -f .env ]; then
    echo -e "Copying .env.example to .env..."
    cp .env.example .env
    echo -e "${YELLOW}⚠️  IMPORTANT: Please ensure your Database credentials in 'backend/.env' are correct!${NC}"
    # Ideally, we would pause here, but for automation we assume default or docker settings
else
    echo -e "✓ .env exists."
fi

# Install Go Deps
echo -e "Installing Go dependencies..."
go mod tidy
go mod download

# Run Migrations
echo -e "Running Database Migrations..."
# Checks if DB connection is valid before running make
if make migrate-up; then
    echo -e "${GREEN}✓ Database migrations applied.${NC}"
else
    echo -e "${RED}❌ Migration failed. Is your PostgreSQL running and configured in .env?${NC}"
    exit 1
fi

cd ..

# 3. Frontend Setup
# ------------------------------------------------------------------------------
echo -e "\n${YELLOW}[3/5] Setting up Frontend...${NC}"
cd frontend || { echo -e "${RED}Error: 'frontend' directory not found.${NC}"; exit 1; }

# Handle .env
if [ ! -f .env.local ]; then
    echo -e "Copying .env.local.example to .env.local..."
    if [ -f .env.local.example ]; then
        cp .env.local.example .env.local
    else 
        # Fallback if example is named differently or missing
        touch .env.local
        echo "NEXT_PUBLIC_SOLANA_RPC=https://api.mainnet-beta.solana.com" >> .env.local
    fi
else
    echo -e "✓ .env.local exists."
fi

# Install Node Deps
echo -e "Installing Node modules (this might take a moment)..."
npm install --silent

cd ..

# 4. Runner Logic
# ------------------------------------------------------------------------------
echo -e "\n${YELLOW}[4/5] Starting Services...${NC}"

# Trap SIGINT (Ctrl+C) to kill background processes
trap "kill 0" EXIT

# Start Backend in background
echo -e "${BLUE}Starting Backend Server...${NC}"
cd backend
go run main.go &
BACKEND_PID=$!
cd ..

# Wait a moment for Backend to initialize
sleep 2

# Start Frontend
echo -e "${BLUE}Starting Frontend Client...${NC}"
cd frontend
npm run dev &
FRONTEND_PID=$!
cd ..

echo -e "\n${GREEN}======================================================${NC}"
echo -e "${GREEN}   ✅ System is up and running!   ${NC}"
echo -e "${GREEN}   - Backend: http://localhost:8080 (or port in .env)${NC}"
echo -e "${GREEN}   - Frontend: http://localhost:3000${NC}"
echo -e "${BLUE}   Press Ctrl+C to stop all services.${NC}"
echo -e "${GREEN}======================================================${NC}"

# Keep script running to maintain background processes
wait

# Cleanup