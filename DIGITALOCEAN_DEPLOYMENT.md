#!/bin/bash
# Fleet Telemetry - DigitalOcean Deployment Guide
# Complete guide to fork, clone, and deploy on DigitalOcean

cat << 'EOF'

╔══════════════════════════════════════════════════════════════════════════════╗
║                                                                              ║
║          FLEET TELEMETRY - DIGITALOCEAN DEPLOYMENT GUIDE                     ║
║                  (PostgreSQL + MySQL Connectors)                             ║
║                                                                              ║
╚══════════════════════════════════════════════════════════════════════════════╝

📋 STEP 1: CREATE A FORK ON GITHUB
══════════════════════════════════════════════════════════════════════════════

Option A: Manual Fork (Recommended)
───────────────────────────────────
1. Go to: https://github.com/teslamotors/fleet-telemetry
2. Click the "Fork" button (top right)
3. Select your GitHub account as the destination
4. GitHub creates: https://github.com/YOUR-USERNAME/fleet-telemetry

Option B: Using GitHub CLI
─────────────────────────
# Install GitHub CLI (if not already installed)
curl -sL https://cli.github.com/install.sh | bash

# Fork the repository
gh repo fork teslamotors/fleet-telemetry --clone

Result: Creates fork in your account + clones to /fleet-telemetry/

══════════════════════════════════════════════════════════════════════════════

📋 STEP 2: PREPARE YOUR FORK
══════════════════════════════════════════════════════════════════════════════

The fork will contain:
  ✅ MySQL Connector (/datastore/mysql/)
  ✅ PostgreSQL Connector (/datastore/postgres/)
  ✅ Docker Compose PostgreSQL (docker-compose.postgres.yml)
  ✅ All documentation files
  ✅ Updated config files

All our changes are already in your local fork!

══════════════════════════════════════════════════════════════════════════════

📋 STEP 3: CREATE A DIGITALOCEAN VM
══════════════════════════════════════════════════════════════════════════════

Option A: Using DigitalOcean Console
────────────────────────────────────
1. Log in to: https://cloud.digitalocean.com
2. Click "Create" → "Droplet"
3. Choose Image: Ubuntu 22.04 LTS (recommended)
4. Choose Plan: Basic ($4-6/month minimum)
   - 1 GB CPU, 1 GB RAM (minimum for Docker)
   - Better: 2 GB CPU, 2 GB RAM for compilation
5. Choose Region: Close to your location
6. Add authentication: SSH key (recommended) or password
7. Hostname: fleet-telemetry-server
8. Click "Create Droplet"

Recommended Specs:
  • Image: Ubuntu 22.04 LTS
  • Plan: Standard ($6/month - 2GB RAM, 1 vCPU)
  • Storage: 50GB SSD
  • Region: Choose nearest to you
  • Authentication: SSH key

Option B: Using DigitalOcean CLI
──────────────────────────────
# Install doctl
cd ~
wget https://github.com/digitalocean/doctl/releases/download/v1.94.0/doctl-1.94.0-linux-x64.tar.gz
tar xf ~/doctl-1.94.0-linux-x64.tar.gz
sudo mv ~/doctl /usr/local/bin

# Authenticate
doctl auth init

# Create Droplet
doctl compute droplet create fleet-telemetry-server \
  --region nyc3 \
  --image ubuntu-22-04-x64 \
  --size s-2vcpu-2gb

══════════════════════════════════════════════════════════════════════════════

📋 STEP 4: SSH INTO YOUR DIGITALOCEAN DROPLET
══════════════════════════════════════════════════════════════════════════════

Find your Droplet IP:
  • Log in to DigitalOcean console
  • Find your droplet
  • Copy the IP address (e.g., 123.45.67.89)

SSH in:
  ssh root@YOUR_DROPLET_IP

Or if you set a username:
  ssh username@YOUR_DROPLET_IP

Test connection:
  # You should see a Linux prompt
  # Example: root@fleet-telemetry-server:~#

══════════════════════════════════════════════════════════════════════════════

📋 STEP 5: INSTALL PREREQUISITES ON DIGITALOCEAN
══════════════════════════════════════════════════════════════════════════════

Connect to your Droplet and run:

# Update system
sudo apt update && sudo apt upgrade -y

# Install Docker
sudo apt install -y docker.io docker-compose

# Install Git
sudo apt install -y git

# Install Go (for certificate generation)
sudo apt install -y golang-go

# Add your user to docker group (to run docker without sudo)
sudo usermod -aG docker $USER

# Verify Docker installation
docker --version
docker-compose --version

# Log out and back in for group changes to take effect
exit
ssh username@YOUR_DROPLET_IP

# Test docker (should work without sudo now)
docker ps

══════════════════════════════════════════════════════════════════════════════

📋 STEP 6: CLONE YOUR FORK ON DIGITALOCEAN
══════════════════════════════════════════════════════════════════════════════

SSH into your Droplet and run:

# Clone your fork (not the original repository!)
git clone https://github.com/YOUR-USERNAME/fleet-telemetry.git
cd fleet-telemetry

# Verify you're on your fork
git remote -v
# Should show your GitHub username, not teslamotors

# Verify the connectors exist
ls -la datastore/mysql/
ls -la datastore/postgres/

# Verify Docker Compose PostgreSQL file
ls -la docker-compose.postgres.yml

════════════════════════════════════════════════════════════════════════════════

📋 STEP 7: GENERATE TLS CERTIFICATES
════════════════════════════════════════════════════════════════════════════════

Still in your Droplet (fleet-telemetry directory):

# Generate certificates
go run tools/main.go

# Verify they were created
ls -la test/integration/test-certs/

Expected files:
  • server.crt (certificate)
  • server.key (private key)

════════════════════════════════════════════════════════════════════════════════

📋 STEP 8: START DOCKER COMPOSE ON DIGITALOCEAN
════════════════════════════════════════════════════════════════════════════════

# Build and start everything
docker-compose -f docker-compose.postgres.yml up -d

# First run takes 5-10 minutes (building dependencies)
# Watch the build progress:
docker-compose -f docker-compose.postgres.yml logs -f

# Wait until you see: "app_1 | " (indicating app is running)
# Press Ctrl+C to exit logs

# Check service status
docker-compose -f docker-compose.postgres.yml ps

# Expected output:
#   NAME                        STATUS          PORTS
#   fleet-telemetry-postgres    Up (healthy)    5432/tcp
#   fleet-telemetry-app         Up              0.0.0.0:443->443/tcp, ...

════════════════════════════════════════════════════════════════════════════════

📋 STEP 9: VERIFY DEPLOYMENT
════════════════════════════════════════════════════════════════════════════════

# Check if app is responding (from your local computer)
curl -k https://YOUR_DROPLET_IP:443/status
# Should return JSON with status info

# Check metrics
curl http://YOUR_DROPLET_IP:8000/metrics | head -20

# Query database from Droplet:
docker exec fleet-telemetry-postgres \
  psql -U telemetry_user -d fleet_telemetry \
  -c "SELECT COUNT(*) FROM tesla_V;"

════════════════════════════════════════════════════════════════════════════════

📋 STEP 10: CONFIGURE FIREWALL (IMPORTANT!)
════════════════════════════════════════════════════════════════════════════════

DigitalOcean Droplet Firewall:
1. Log in to DigitalOcean console
2. Go to "Networking" → "Firewalls"
3. Create new firewall (name: fleet-telemetry-fw)
4. Add inbound rules:
   • HTTPS (443) from anywhere
   • HTTP (8080) from your IP (status endpoint)
   • HTTP (8000) from your IP (metrics - optional)
5. Add outbound rules:
   • All TCP/UDP to anywhere
6. Apply to your Droplet

Alternative: Using DigitalOcean CLI
──────────────────────────────────
doctl compute firewall create \
  --name fleet-telemetry-fw \
  --inbound-rules "protocol:tcp,ports:443,sources:addresses:0.0.0.0/0,::/0" \
  --inbound-rules "protocol:tcp,ports:8080,sources:addresses:YOUR_IP/32" \
  --outbound-rules "protocol:tcp,ports:all,destinations:addresses:0.0.0.0/0,::/0" \
  --outbound-rules "protocol:udp,ports:all,destinations:addresses:0.0.0.0/0,::/0"

════════════════════════════════════════════════════════════════════════════════

📋 STEP 11: ACCESS YOUR SERVICES
════════════════════════════════════════════════════════════════════════════════

From your local computer:

Status Endpoint:
  curl -k https://YOUR_DROPLET_IP:443/status

Metrics (Prometheus):
  curl http://YOUR_DROPLET_IP:8000/metrics

Database (from Droplet terminal):
  psql -h localhost -U telemetry_user -d fleet_telemetry

════════════════════════════════════════════════════════════════════════════════

📋 STEP 12: PERSISTENT STORAGE & BACKUPS (OPTIONAL)
════════════════════════════════════════════════════════════════════════════════

Your PostgreSQL data is stored in:
  Docker volume: postgres_data

To backup data:
  docker run --rm -v fleet-telemetry_postgres_data:/data \
    -v /tmp:/backup \
    busybox tar czf /backup/postgres-backup-$(date +%Y%m%d).tar.gz -C /data .

To restore from backup:
  docker-compose -f docker-compose.postgres.yml down -v
  # Then restore the volume from backup
  docker run --rm -v fleet-telemetry_postgres_data:/data \
    -v /tmp:/backup \
    busybox tar xzf /backup/postgres-backup-YYYYMMDD.tar.gz -C /data

Consider DigitalOcean Volumes for persistent storage:
  1. Create a Volume in DigitalOcean console
  2. Attach to your Droplet
  3. Mount at /mnt/telemetry-data
  4. Update docker-compose to use this path

════════════════════════════════════════════════════════════════════════════════

📋 STEP 13: MONITORING & LOGS
════════════════════════════════════════════════════════════════════════════════

View logs:
  docker-compose -f docker-compose.postgres.yml logs -f app
  docker-compose -f docker-compose.postgres.yml logs -f postgres

Check disk usage:
  df -h

Check memory usage:
  free -h

Monitor Docker:
  docker stats

Check system resources:
  top

════════════════════════════════════════════════════════════════════════════════

📋 TROUBLESHOOTING COMMON ISSUES
════════════════════════════════════════════════════════════════════════════════

Issue: "docker-compose: command not found"
Solution:
  sudo apt install -y docker-compose

Issue: "Permission denied: docker"
Solution:
  sudo usermod -aG docker $USER
  exit (and log back in)

Issue: "Droplet: Out of memory during build"
Solution:
  • Upgrade to larger Droplet (2GB RAM minimum)
  • Or build locally and push image to registry

Issue: "Port 443 already in use"
Solution:
  sudo lsof -i :443
  Kill process: sudo kill -9 <PID>
  Or change port in docker-compose.postgres.yml

Issue: "Cannot connect to database"
Solution:
  docker-compose -f docker-compose.postgres.yml logs postgres

Issue: "App won't start"
Solution:
  docker-compose -f docker-compose.postgres.yml logs app
  Check if postgres is healthy (should say "healthy")

════════════════════════════════════════════════════════════════════════════════

📋 UPDATING YOUR FORK
════════════════════════════════════════════════════════════════════════════════

If you make changes locally and want to push to your fork:

# Make changes
vim datastore/postgres/postgres.go

# Commit changes
git add .
git commit -m "Your commit message"

# Push to your fork
git push origin main
# Or: git push origin YOUR_BRANCH_NAME

On DigitalOcean, pull the latest:

docker-compose -f docker-compose.postgres.yml down
git pull origin main
docker-compose -f docker-compose.postgres.yml up -d --build

════════════════════════════════════════════════════════════════════════════════

📋 USEFUL DIGITALOCEAN DROPLET COMMANDS
════════════════════════════════════════════════════════════════════════════════

# List your droplets
doctl compute droplet list

# Get droplet info
doctl compute droplet get DROPLET_ID --format Name,PublicIPv4,Status

# Reboot a droplet
doctl compute droplet-action reboot DROPLET_ID

# Snapshot a droplet (for backup)
doctl compute droplet-action snapshot DROPLET_ID --snapshot-name backup-$(date +%Y%m%d)

# Resize droplet
doctl compute droplet-action resize DROPLET_ID --size s-4vcpu-8gb

════════════════════════════════════════════════════════════════════════════════

📋 RECOMMENDED DIGITALOCEAN SETUP FOR PRODUCTION
════════════════════════════════════════════════════════════════════════════════

Droplet Configuration:
  • Image: Ubuntu 22.04 LTS
  • Size: Standard ($18/month - 4GB RAM, 2 vCPU)
  • Region: Closest to users
  • Backups: Enabled

Additional Services:
  • DigitalOcean Volumes (for persistent PostgreSQL data)
  • DigitalOcean Spaces (for log backups)
  • DigitalOcean Monitoring (track CPU, memory, disk)
  • DigitalOcean Firewall (restrict access)

Recommended safeguards:
  • Enable automatic backups
  • Set up monitoring alerts
  • Configure email notifications
  • Use SSH keys (not passwords)

════════════════════════════════════════════════════════════════════════════════

📋 SECURITY BEST PRACTICES
════════════════════════════════════════════════════════════════════════════════

1. Change PostgreSQL password in docker-compose.postgres.yml
   • Current: telemetry_password
   • Generate strong password: openssl rand -base64 32

2. Update TLS certificates
   • Use real certificates from Let's Encrypt
   • Or use your own CA certificates

3. Firewall rules
   • Only open necessary ports (443, 8080, 8000)
   • Restrict by IP when possible

4. SSH security
   • Use SSH keys, disable password auth
   • Change default SSH port (optional)

5. Keep system updated
   sudo apt update && sudo apt upgrade -y

════════════════════════════════════════════════════════════════════════════════

✅ YOU'RE DONE!
════════════════════════════════════════════════════════════════════════════════

Your Fleet Telemetry deployment is now running on DigitalOcean!

Services available:
  ✅ PostgreSQL (5432)
  ✅ Fleet Telemetry (443)
  ✅ Status Endpoint (8080)
  ✅ Metrics (8000)

Next steps:
  1. Send test data to your Fleet Telemetry instance
  2. Query data from PostgreSQL
  3. Monitor metrics
  4. Set up backups

For more info:
  • Fleet Telemetry Docs: POSTGRES_SETUP.md
  • DigitalOcean Docs: https://docs.digitalocean.com/
  • Docker Compose Docs: https://docs.docker.com/compose/

═══════════════════════════════════════════════════════════════════════════════

EOF

