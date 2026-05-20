# Fork and Deploy to DigitalOcean - Quick Reference

## 🍴 Step 1: Fork on GitHub (5 minutes)

### Option A: Manual (Recommended)
1. Go to: https://github.com/teslamotors/fleet-telemetry
2. Click **"Fork"** button (top right)
3. Select your account
4. New fork created at: `https://github.com/YOUR-USERNAME/fleet-telemetry`

### Option B: GitHub CLI
```bash
gh repo fork teslamotors/fleet-telemetry --clone
```

---

## 🐳 Step 2: Create DigitalOcean Droplet (10 minutes)

### Via Console (easiest)
1. Go to: https://cloud.digitalocean.com
2. Click **"Create"** → **"Droplet"**
3. Choose:
   - **Image**: Ubuntu 22.04 LTS
   - **Plan**: Standard $6/month (2GB RAM, 1 vCPU)
   - **Region**: Closest to you
   - **Auth**: SSH key (recommended)
   - **Hostname**: fleet-telemetry-server
4. Click **"Create Droplet"** and wait ~1 minute

### Via CLI
```bash
doctl compute droplet create fleet-telemetry-server \
  --region nyc3 \
  --image ubuntu-22-04-x64 \
  --size s-2vcpu-2gb
```

---

## 🔑 Step 3: SSH into Your Droplet (1 minute)

### Get IP Address
- DigitalOcean console → Droplets → Copy IP
- Or: `doctl compute droplet list`

### Connect
```bash
ssh root@YOUR_DROPLET_IP
# Or if you set a username:
ssh ubuntu@YOUR_DROPLET_IP
```

---

## ⚡ Step 4: Automated Setup (10 minutes)

### Download and Run Setup Script
```bash
# SSH into your droplet, then:

# Clone your fork temporarily to get the setup script
git clone https://github.com/YOUR-USERNAME/fleet-telemetry.git temp-setup
cd temp-setup

# Run the automated setup script
bash digitalocean_setup.sh YOUR-USERNAME

# That's it! The script does everything:
#  ✅ Updates system
#  ✅ Installs Docker/Docker Compose
#  ✅ Clones your fork
#  ✅ Generates TLS certificates
#  ✅ Builds and starts Docker Compose
```

---

## 📋 Step 5: Manual Setup (if automated doesn't work)

### Install Prerequisites
```bash
sudo apt update && sudo apt upgrade -y
sudo apt install -y docker.io docker-compose git golang-go
sudo usermod -aG docker $USER
exit  # Log out and back in
```

### Clone Your Fork
```bash
git clone https://github.com/YOUR-USERNAME/fleet-telemetry.git
cd fleet-telemetry
```

### Generate Certificates
```bash
go run tools/main.go
```

### Start Docker Compose
```bash
docker-compose -f docker-compose.postgres.yml up -d
docker-compose -f docker-compose.postgres.yml ps
```

---

## ✅ Step 6: Verify It Works

### Check Status
```bash
# From your local computer:
curl -k https://YOUR_DROPLET_IP:443/status

# From your droplet:
docker-compose -f docker-compose.postgres.yml ps
```

### Check Logs
```bash
docker-compose -f docker-compose.postgres.yml logs -f app
```

### Query Database
```bash
docker exec fleet-telemetry-postgres \
  psql -U telemetry_user -d fleet_telemetry \
  -c "SELECT COUNT(*) FROM tesla_V;"
```

---

## 🔒 Step 7: Security - Configure Firewall

### Via DigitalOcean Console
1. **Networking** → **Firewalls**
2. Create new firewall
3. **Inbound Rules**:
   - HTTPS (443) from anywhere
   - HTTP (8080) from your IP only
4. **Outbound Rules**:
   - All TCP/UDP to anywhere
5. Apply to your Droplet

### Via CLI
```bash
doctl compute firewall create \
  --name fleet-telemetry-fw \
  --inbound-rules "protocol:tcp,ports:443,sources:addresses:0.0.0.0/0" \
  --inbound-rules "protocol:tcp,ports:8080,sources:addresses:YOUR_IP/32" \
  --outbound-rules "protocol:tcp,ports:all,destinations:addresses:0.0.0.0/0"
```

---

## 📚 Useful Commands

### Monitor Your Services
```bash
docker-compose -f docker-compose.postgres.yml logs -f
docker stats
```

### Rebuild After Code Changes
```bash
git pull origin main
docker-compose -f docker-compose.postgres.yml up -d --build
```

### Stop All Services
```bash
docker-compose -f docker-compose.postgres.yml down
```

### Remove Everything (including data!)
```bash
docker-compose -f docker-compose.postgres.yml down -v
```

---

## 🎯 Summary

| Step | Time | Command |
|------|------|---------|
| 1. Fork | 5 min | Click "Fork" on GitHub |
| 2. Create Droplet | 10 min | DigitalOcean console |
| 3. SSH | 1 min | `ssh root@IP` |
| 4. Auto Setup | 10 min | `bash digitalocean_setup.sh YOUR-USER` |
| 5. Verify | 2 min | `curl -k https://IP:443/status` |
| 6. Firewall | 3 min | DigitalOcean console |
| **Total** | **~30 min** | |

---

## 📖 Full Documentation

- **Detailed Setup**: `DIGITALOCEAN_DEPLOYMENT.md`
- **PostgreSQL**: `POSTGRES_SETUP.md`
- **Docker Build**: `DOCKER_BUILD_SETUP.md`
- **Database Comparison**: `POSTGRES_VS_MYSQL.md`

---

## ❓ Troubleshooting

### Build fails on Droplet
- **Problem**: Not enough memory
- **Solution**: Upgrade to $12/month plan (4GB RAM)

### Can't find your IP
```bash
# From your droplet:
hostname -I

# Or from local computer:
doctl compute droplet list
```

### Services won't start
```bash
# Check logs
docker-compose -f docker-compose.postgres.yml logs postgres
docker-compose -f docker-compose.postgres.yml logs app
```

### Can't connect from local computer
- Check DigitalOcean firewall allows port 443
- Check your firewall/ISP allows HTTPS outbound
- Verify Droplet is running: `doctl compute droplet list`

---

## 🚀 You're Done!

Your Fleet Telemetry is now running on DigitalOcean with:
- ✅ PostgreSQL (5432)
- ✅ MySQL connector (ready to use)
- ✅ Fleet Telemetry (443 HTTPS)
- ✅ Status endpoint (8080 HTTP)
- ✅ Metrics (8000 HTTP)

Start sending data to your Fleet Telemetry server!

---

**Need more help?** See `DIGITALOCEAN_DEPLOYMENT.md` for detailed instructions.

