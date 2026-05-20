#!/bin/bash
# Fleet Telemetry - DigitalOcean Automated Setup Script
# Run this on your DigitalOcean droplet to set everything up automatically

set -e  # Exit on error

echo "╔══════════════════════════════════════════════════════════════════════════════╗"
echo "║                                                                              ║"
echo "║          Fleet Telemetry - DigitalOcean Automated Setup                      ║"
echo "║                                                                              ║"
echo "╚══════════════════════════════════════════════════════════════════════════════╝"
echo ""

# Configuration
GITHUB_USERNAME="${1:-}"
REPO_URL="https://github.com/${GITHUB_USERNAME}/fleet-telemetry.git"

if [ -z "$GITHUB_USERNAME" ]; then
    echo "❌ ERROR: GitHub username required"
    echo ""
    echo "Usage: bash digitalocean_setup.sh YOUR_GITHUB_USERNAME"
    echo ""
    echo "Example:"
    echo "  bash digitalocean_setup.sh johndoe"
    echo ""
    exit 1
fi

echo "🚀 Starting automated setup..."
echo "   GitHub Username: $GITHUB_USERNAME"
echo "   Repository: $REPO_URL"
echo ""

# Step 1: Update system
echo "📦 Step 1: Updating system packages..."
sudo apt update
sudo apt upgrade -y
echo "✅ System updated"
echo ""

# Step 2: Install Docker
echo "🐳 Step 2: Installing Docker & Docker Compose..."
sudo apt install -y docker.io docker-compose git golang-go
echo "✅ Docker installed"
echo ""

# Step 3: Configure Docker user
echo "👤 Step 3: Configuring Docker user access..."
sudo usermod -aG docker $USER
echo "✅ Docker user configured"
echo ""

# Step 4: Verify Docker installation
echo "✔️ Step 4: Verifying Docker installation..."
docker --version
docker-compose --version
echo "✅ Docker verified"
echo ""

# Step 5: Clone repository
echo "📥 Step 5: Cloning your fork..."
if [ -d "fleet-telemetry" ]; then
    echo "⚠️  Directory already exists, removing..."
    rm -rf fleet-telemetry
fi
git clone "$REPO_URL"
cd fleet-telemetry
echo "✅ Repository cloned"
echo ""

# Step 6: Verify files
echo "🔍 Step 6: Verifying Fleet Telemetry files..."
if [ -f "docker-compose.postgres.yml" ]; then
    echo "✅ docker-compose.postgres.yml found"
else
    echo "❌ docker-compose.postgres.yml not found"
    exit 1
fi

if [ -d "datastore/postgres" ]; then
    echo "✅ PostgreSQL connector found"
else
    echo "❌ PostgreSQL connector not found"
    exit 1
fi

if [ -d "datastore/mysql" ]; then
    echo "✅ MySQL connector found"
else
    echo "❌ MySQL connector not found"
    exit 1
fi
echo ""

# Step 7: Generate TLS certificates
echo "🔐 Step 7: Generating TLS certificates..."
if [ ! -d "test/integration/test-certs" ]; then
    go run tools/main.go > /dev/null 2>&1 || true
    sleep 2
fi

if [ -f "test/integration/test-certs/server.crt" ] && [ -f "test/integration/test-certs/server.key" ]; then
    echo "✅ TLS certificates generated"
else
    echo "⚠️  Warning: TLS certificates might not have generated properly"
fi
echo ""

# Step 8: Verify PostgreSQL config
echo "📄 Step 8: Verifying PostgreSQL configuration..."
if [ -f "test/integration/postgres_config.json" ]; then
    echo "✅ postgres_config.json found"
else
    echo "❌ postgres_config.json not found"
    exit 1
fi
echo ""

# Step 9: Build and start Docker Compose
echo "🏗️  Step 9: Building Docker images (this takes 5-10 minutes on first run)..."
echo "    Building libsodium, libzmq, and Fleet Telemetry..."
docker-compose -f docker-compose.postgres.yml up -d

# Wait for services to be ready
echo ""
echo "⏳ Waiting for services to be ready..."
sleep 30

# Step 10: Verify services
echo "✔️  Step 10: Verifying services..."
echo ""

if docker-compose -f docker-compose.postgres.yml ps | grep -q "Up"; then
    echo "✅ Services are running:"
    docker-compose -f docker-compose.postgres.yml ps
else
    echo "⚠️  Warning: Services might not be fully started yet"
    echo "    Check status with: docker-compose -f docker-compose.postgres.yml ps"
fi
echo ""

# Step 11: Print next steps
echo "╔══════════════════════════════════════════════════════════════════════════════╗"
echo "║                                                                              ║"
echo "║                    ✅ SETUP COMPLETE!                                        ║"
echo "║                                                                              ║"
echo "╚══════════════════════════════════════════════════════════════════════════════╝"
echo ""
echo "Your Fleet Telemetry server is now running on DigitalOcean!"
echo ""
echo "📊 Services Status:"
docker-compose -f docker-compose.postgres.yml ps
echo ""
echo "🔗 Next Steps:"
echo ""
echo "1. Get your Droplet IP address:"
echo "   doctl compute droplet list"
echo "   Or check DigitalOcean console"
echo ""
echo "2. Test the API from your local computer:"
echo "   curl -k https://YOUR_DROPLET_IP:443/status"
echo ""
echo "3. Configure DigitalOcean Firewall:"
echo "   • Allow HTTPS (443) from anywhere"
echo "   • Allow HTTP (8080) from your IP for status"
echo "   • Allow any outbound traffic"
echo ""
echo "4. Monitor logs:"
echo "   docker-compose -f docker-compose.postgres.yml logs -f app"
echo ""
echo "5. Query the database:"
echo "   docker exec fleet-telemetry-postgres psql -U telemetry_user -d fleet_telemetry -c 'SELECT COUNT(*) FROM tesla_V;'"
echo ""
echo "📚 Full documentation:"
echo "   cat /home/ubuntu/fleet-telemetry/DIGITALOCEAN_DEPLOYMENT.md"
echo ""
echo "🔒 IMPORTANT - Security Recommendations:"
echo "   1. Change PostgreSQL password in docker-compose.postgres.yml"
echo "   2. Use real TLS certificates (Let's Encrypt recommended)"
echo "   3. Enable DigitalOcean firewall"
echo "   4. Set up SSH key authentication (disable passwords)"
echo ""
echo "═══════════════════════════════════════════════════════════════════════════════"
echo ""

