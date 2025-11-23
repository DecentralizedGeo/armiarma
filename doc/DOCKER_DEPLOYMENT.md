# Docker Deployment Guide

## Architecture: One Container Per Network

The armiarma crawler uses a **one-container-per-network** architecture for better isolation, stability, and resource management.

### Architecture Diagram

```
┌─────────────────────────────────────────────────────────────┐
│                    Docker Compose Stack                      │
├─────────────────────────────────────────────────────────────┤
│                                                               │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐      │
│  │   Ethereum   │  │   Polygon    │  │   Future     │      │
│  │   Crawler    │  │   Crawler    │  │   Networks   │      │
│  │   :9080      │  │   :9081      │  │              │      │
│  └──────┬───────┘  └──────┬───────┘  └──────┬───────┘      │
│         │                  │                  │              │
│         └──────────┬───────┴──────────────────┘              │
│                    │                                          │
│         ┌──────────▼──────────┐                              │
│         │    PostgreSQL       │                              │
│         │    Database         │                              │
│         │    :5432            │                              │
│         └──────────┬──────────┘                              │
│                    │                                          │
│         ┌──────────▼──────────┐                              │
│         │    Prometheus       │◄─────────┐                   │
│         │    :9090            │          │                   │
│         └──────────┬──────────┘          │                   │
│                    │                     │                   │
│         ┌──────────▼──────────┐          │                   │
│         │     Grafana         │──────────┘                   │
│         │     :3000           │                              │
│         └─────────────────────┘                              │
│                                                               │
└─────────────────────────────────────────────────────────────┘
```

## Why One Container Per Network?

### ✅ Advantages

1. **Fault Isolation**: If Polygon crashes, Ethereum keeps running
2. **Independent Scaling**: Allocate different resources per network
3. **Flexible Configuration**: Different settings per network
4. **Easy Deployment**: Start/stop/update networks independently
5. **Better Monitoring**: Separate metrics endpoints per network
6. **Debugging**: Easier to isolate network-specific issues

### ⚠️ Trade-offs

- Slightly more RAM usage (~50-100MB per container)
- More configuration (but organized and clear)
- Multiple service definitions (but more maintainable)

## Quick Start

### 1. Copy Environment Template

```bash
cp .env.example .env
```

### 2. Edit Configuration

Edit `.env` file:

```bash
# Database
DB_PORT=5432

# Ethereum Crawler
ETH_LOG_LEVEL=info
ETH_FORK_DIGEST=0x6a95a1a9  # Deneb mainnet
ETH_PORT=9020
ETH_METRICS_PORT=9080

# Polygon Crawler
POLYGON_LOG_LEVEL=info
POLYGON_PORT=30303
POLYGON_METRICS_PORT=9081
```

### 3. Start All Services

```bash
docker-compose up -d
```

### 4. Start Specific Network

```bash
# Just Ethereum
docker-compose up -d db prometheus grafana eth_crawler

# Just Polygon
docker-compose up -d db prometheus grafana polygon_crawler

# Both
docker-compose up -d
```

## Service Configuration

### Ethereum Crawler

| Variable | Default | Description |
|----------|---------|-------------|
| `ETH_LOG_LEVEL` | info | Log level (trace, debug, info, warn, error) |
| `ETH_PSQL_ENDP` | Auto-configured | PostgreSQL connection string |
| `ETH_PEERS_BACKUP` | 30m | Peer snapshot interval |
| `ETH_FORK_DIGEST` | - | Ethereum fork digest |
| `ETH_PORT` | 9020 | P2P port |
| `ETH_METRICS_PORT` | 9080 | Prometheus metrics port |
| `ETH_SSE_PORT` | 9099 | Server-sent events port |
| `ETH_PERSIST_CONNEVENTS` | false | Store connection events |

### Polygon Crawler

| Variable | Default | Description |
|----------|---------|-------------|
| `POLYGON_LOG_LEVEL` | info | Log level |
| `POLYGON_PSQL_ENDP` | Auto-configured | PostgreSQL connection string |
| `POLYGON_PEERS_BACKUP` | 30m | Peer snapshot interval |
| `POLYGON_PORT` | 30303 | P2P discovery port (UDP & TCP) |
| `POLYGON_METRICS_PORT` | 9081 | Prometheus metrics port |
| `POLYGON_PERSIST_CONNEVENTS` | false | Store connection events |

## Port Mapping

| Service | Internal Port | External Port | Protocol | Purpose |
|---------|---------------|---------------|----------|---------|
| PostgreSQL | 5432 | 5432 | TCP | Database |
| Prometheus | 9090 | 9090 | TCP | Metrics storage |
| Grafana | 3000 | 3000 | TCP | Visualization |
| Ethereum P2P | 9020 | 9020 | TCP | P2P connections |
| Ethereum Metrics | 9080 | 9080 | TCP | Prometheus scrape |
| Ethereum SSE | 9099 | 9099 | TCP | Event stream |
| Polygon P2P | 30303 | 30303 | UDP+TCP | P2P discovery & connections |
| Polygon Metrics | 9080 | 9081 | TCP | Prometheus scrape |

**Note:** Polygon uses different ports to avoid conflicts with Ethereum.

## Resource Management

### Set Resource Limits

Add to specific crawler in `docker-compose.yaml`:

```yaml
polygon_crawler:
  # ... existing config ...
  deploy:
    resources:
      limits:
        cpus: '1.0'
        memory: 1G
      reservations:
        cpus: '0.5'
        memory: 512M
```

### Recommended Resources

| Service | CPU | Memory | Notes |
|---------|-----|--------|-------|
| Ethereum Crawler | 1-2 cores | 2-4 GB | Higher for gossipsub |
| Polygon Crawler | 0.5-1 core | 512MB-1GB | Lower discovery overhead |
| PostgreSQL | 1-2 cores | 2-4 GB | Depends on data volume |
| Prometheus | 0.5-1 core | 1-2 GB | Depends on retention |
| Grafana | 0.25-0.5 core | 256-512 MB | Light visualization |

## Management Commands

### View Logs

```bash
# All services
docker-compose logs -f

# Specific service
docker-compose logs -f polygon_crawler
docker-compose logs -f eth_crawler

# Last 100 lines
docker-compose logs --tail=100 polygon_crawler
```

### Restart Services

```bash
# Restart Polygon crawler
docker-compose restart polygon_crawler

# Restart Ethereum crawler
docker-compose restart eth_crawler

# Restart all
docker-compose restart
```

### Stop Services

```bash
# Stop specific network
docker-compose stop polygon_crawler

# Stop all
docker-compose down

# Stop and remove volumes (CAUTION: deletes data)
docker-compose down -v
```

### Update Crawler

```bash
# Rebuild image
docker-compose build

# Restart with new image
docker-compose up -d --force-recreate polygon_crawler
```

## Monitoring

### Prometheus Metrics

Each crawler exposes Prometheus metrics:

- **Ethereum**: http://localhost:9080/metrics
- **Polygon**: http://localhost:9081/metrics

### Grafana Dashboards

Access at: http://localhost:3000

Default credentials:
- Username: `admin`
- Password: `admin` (change on first login)

### Health Checks

```bash
# Check container status
docker-compose ps

# Check Ethereum crawler
curl http://localhost:9080/metrics | grep discovery_

# Check Polygon crawler
curl http://localhost:9081/metrics | grep discovery_

# Check database
docker-compose exec db psql -U user -d armiarmadb -c "SELECT network, COUNT(*) FROM peer_info GROUP BY network;"
```

## Troubleshooting

### Polygon Crawler Not Discovering Peers

```bash
# Check logs
docker-compose logs polygon_crawler | grep -i discovery

# Verify bootnodes are reachable
docker-compose exec polygon_crawler ping -c 3 3.36.224.80

# Check UDP port is open
docker-compose exec polygon_crawler nc -vzu 3.36.224.80 30303
```

### High Memory Usage

```bash
# Check resource usage
docker stats

# If memory is too high, add limits:
# Edit docker-compose.yaml and add deploy.resources section
```

### Database Connection Issues

```bash
# Check database is healthy
docker-compose ps db

# Test connection
docker-compose exec polygon_crawler psql "postgresql://user:password@db:5432/armiarmadb" -c "\dt"

# Check environment variables
docker-compose exec polygon_crawler env | grep PSQL
```

### Port Conflicts

If ports are already in use:

```bash
# Check what's using the port
sudo netstat -tlnp | grep 30303

# Change port in .env file
echo "POLYGON_PORT=30304" >> .env

# Restart
docker-compose up -d polygon_crawler
```

## Data Persistence

### Volume Locations

- **PostgreSQL**: `./app-data/postgresql_db`
- **Prometheus**: `./app-data/prometheus_db`

### Backup Database

```bash
# Backup
docker-compose exec db pg_dump -U user armiarmadb > backup.sql

# Restore
docker-compose exec -T db psql -U user armiarmadb < backup.sql
```

### Export Data

```bash
# Copy export script to container
docker-compose exec polygon_crawler /bin/bash

# Or run directly
docker-compose exec -e ARMIARMA_DB_HOST=db -e ARMIARMA_DB_USER=user -e ARMIARMA_DB_NAME=armiarmadb \
  polygon_crawler /crawler/scripts/export_polygon_data.sh
```

## Adding New Networks

To add a new network (e.g., Filecoin):

1. Add service to `docker-compose.yaml`:

```yaml
filecoin_crawler:
  image: "armiarma:latest"
  container_name: armiarma_filecoin
  command: |
    filecoin
    --log-level=${FILECOIN_LOG_LEVEL:-info}
    --psql-endpoint=${FILECOIN_PSQL_ENDP:-postgresql://user:password@db:5432/armiarmadb}
  restart: unless-stopped
  depends_on: 
    db:
      condition: service_healthy
  networks: [ cluster ]
  ports: 
    - "${FILECOIN_PORT:-4001}:4001"
    - "127.0.0.1:${FILECOIN_METRICS_PORT:-9082}:9080"
```

2. Add to Prometheus config (`prometheus/docker-prometheus.yml`):

```yaml
- job_name: 'filecoin_crawler'
  static_configs:
    - targets: ['filecoin_crawler:9080']
      labels:
        network: 'filecoin'
```

3. Add environment variables to `.env`:

```bash
FILECOIN_LOG_LEVEL=info
FILECOIN_PORT=4001
FILECOIN_METRICS_PORT=9082
```

4. Start the new crawler:

```bash
docker-compose up -d filecoin_crawler
```

## Production Considerations

### Security

1. **Change default credentials**:
   ```yaml
   environment:
     - POSTGRES_PASSWORD=<strong-password>
   ```

2. **Use secrets for production**:
   ```yaml
   secrets:
     - postgres_password
   ```

3. **Restrict port bindings**:
   ```yaml
   ports:
     - "127.0.0.1:9080:9080"  # Only localhost
   ```

### High Availability

1. **Add restart policies**:
   ```yaml
   restart: unless-stopped
   ```

2. **Health checks**:
   ```yaml
   healthcheck:
     test: ["CMD", "curl", "-f", "http://localhost:9080/metrics"]
     interval: 30s
     timeout: 10s
     retries: 3
   ```

3. **Resource reservations**:
   ```yaml
   deploy:
     resources:
       reservations:
         cpus: '0.5'
         memory: 512M
   ```

### Logging

Use log aggregation in production:

```yaml
logging:
  driver: "json-file"
  options:
    max-size: "10m"
    max-file: "3"
```

Or use centralized logging:

```yaml
logging:
  driver: "syslog"
  options:
    syslog-address: "tcp://logs.example.com:514"
```

## Summary

The **one-container-per-network** approach provides:

✅ Better fault isolation  
✅ Independent resource management  
✅ Flexible configuration  
✅ Easy deployment control  
✅ Clear monitoring  

With minimal downsides (slightly more resources, more configuration).

This architecture scales well as you add more networks and provides the operational flexibility needed for production deployments.

