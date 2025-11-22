# Environment Configuration

This project now uses **per-crawler environment files** for better organization and maintainability.

## File Structure

```
.env.common       # Shared settings (database, etc.)
.env.ethereum     # Ethereum-specific settings
.env.polygon      # Polygon-specific settings
```

## Setup

```bash
# Copy template files to create your configuration
cp .env_template.common .env.common
cp .env_template.ethereum .env.ethereum
cp .env_template.polygon .env.polygon

# Edit as needed
nano .env.common
nano .env.ethereum
nano .env.polygon
```

## Benefits

1. **Uses upstream variable names**: `CRAWLER_LOG_LEVEL`, `CRAWLER_PORT`, etc. (from `.env_template`)
2. **No network prefixes needed**: Each crawler has its own file, so no need for `ETH_*` or `POLYGON_*` prefixes
3. **Easy to add new networks**: Just create `.env.newnetwork` and add to `docker-compose.yaml`
4. **Cleaner configuration**: Related settings grouped together

## How It Works

Docker Compose loads environment variables in order:

```yaml
eth_crawler:
  env_file:
    - .env.common    # Loaded first
    - .env.ethereum  # Loaded second (overrides common)
```

Variables in `.env.ethereum` override those in `.env.common`.

## Variable Reference

### Common Variables (`.env.common`)
- `CRAWLER_PSQL_ENDP` - PostgreSQL connection string
- `DB_HOST`, `DB_PORT`, `DB_NAME` - Database connection details

### Ethereum Variables (`.env.ethereum`)
- `CRAWLER_LOG_LEVEL` - Log level (debug, info, warn, error)
- `CRAWLER_PORT` - P2P port (default: 9020)
- `CRAWLER_METRICS_PORT` - Prometheus metrics port (default: 9080)
- `CRAWLER_SSE_PORT` - Server-sent events port (default: 9099)
- `CRAWLER_PEERS_BACKUP` - Peer backup interval (default: 30m)
- `CRAWLER_FORK_DIGEST` - Ethereum fork digest
- `CRAWLER_GOSSIP_TOPIC` - GossipSub topics to monitor
- `CRAWLER_SUBNET` - Subnets to monitor
- `CRAWLER_PERSIST_CONNEVENTS` - Persist connection events (true/false)

### Polygon Variables (`.env.polygon`)
- `CRAWLER_LOG_LEVEL` - Log level
- `CRAWLER_PORT` - P2P port (default: 30303)
- `CRAWLER_METRICS_PORT` - Prometheus metrics port (default: 9081)
- `CRAWLER_PEERS_BACKUP` - Peer backup interval (default: 30m)
- `CRAWLER_PERSIST_CONNEVENTS` - Persist connection events (true/false)

## Migration from Old Configuration

If you have an existing `.env` file with `ETH_*` and `POLYGON_*` prefixes, you can migrate by:

1. Create `.env.common` with shared database settings
2. Create `.env.ethereum` with `CRAWLER_*` variables (remove `ETH_` prefix)
3. Create `.env.polygon` with `CRAWLER_*` variables (remove `POLYGON_` prefix)

Example migration:
```bash
# Old .env
ETH_LOG_LEVEL=info
ETH_PORT=9020

# New .env.ethereum
CRAWLER_LOG_LEVEL=info
CRAWLER_PORT=9020
```

