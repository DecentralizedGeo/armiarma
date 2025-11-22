# Polygon Crawler Documentation

## Overview

The Polygon crawler extends armiarma to support crawling the Polygon (Bor) network using discv4 peer discovery. This allows continuous discovery and geolocation of Polygon validator nodes.

## Architecture

The Polygon crawler consists of:

1. **discv4 Discovery Service** (`pkg/discovery/discv4/`) - Implements discovery protocol for Geth-based networks
2. **Polygon Network Module** (`pkg/networks/polygon/`) - Network-specific configuration
3. **Polygon Crawler** (`pkg/crawler/polygon.go`) - Main crawler implementation
4. **CLI Command** (`cmd/polygon_crawler.go`) - Command-line interface

## Usage

### Prerequisites

Before building, ensure dependencies are up to date:

```bash
go mod download
```

### Building

```bash
go build -o armiarma .
```

### Running the Crawler

Basic usage:

```bash
./armiarma polygon --psql-endpoint "postgresql://user:pass@localhost:5432/armiarma"
```

### Configuration Options

| Flag | Description | Default | Environment Variable |
|------|-------------|---------|---------------------|
| `--log-level` | Verbosity level (trace, debug, info, warn, error) | info | `ARMIARMA_LOG_LEVEL` |
| `--priv-key` | Private key for the crawler node | Generated | `ARMIARMA_PRIV_KEY` |
| `--ip` | IP address to bind to | 0.0.0.0 | `ARMIARMA_IP` |
| `--port` | TCP/UDP port for P2P | 9000 | `ARMIARMA_PORT` |
| `--metrics-ip` | IP for Prometheus metrics | 0.0.0.0 | `ARMIARMA_METRICS_IP` |
| `--metrics-port` | Port for Prometheus metrics | 9090 | `ARMIARMA_METRICS_PORT` |
| `--user-agent` | User agent string | armiarma | `ARMIARMA_USER_AGENT` |
| `--psql-endpoint` | PostgreSQL connection string | Required | `ARMIARMA_PSQL` |
| `--peers-backup` | Peer backup interval | 30m | `ARMIARMA_BACKUP_INTERVAL` |
| `--bootnode` | Bootnode ENR (can specify multiple) | Built-in | `ARMIARMA_BOOTNODES` |
| `--persist-connevents` | Persist connection events to DB | false | `ARMIARMA_PERSIST_CONNEVENTS` |

### Example with Custom Configuration

```bash
./armiarma polygon \
  --log-level debug \
  --port 30303 \
  --psql-endpoint "postgresql://user:pass@localhost:5432/armiarma" \
  --peers-backup 1h \
  --persist-connevents
```

### Using Environment Variables

```bash
export ARMIARMA_LOG_LEVEL=debug
export ARMIARMA_PORT=30303
export ARMIARMA_PSQL="postgresql://user:pass@localhost:5432/armiarma"
./armiarma polygon
```

## Database Schema

The Polygon crawler uses the same database schema as the Ethereum crawler:

- **peer_info**: Node identification and metadata
- **ips**: IP geolocation data
- **conn_events**: Connection events (if enabled)
- **active_peers**: Periodic snapshots of active peers

All Polygon nodes are stored with `network='Polygon'` in the `peer_info` table.

## Exporting Validator IPs for GeoIP Analysis

### Using the Unified Export Script (Recommended)

The simplest way to export Polygon data is using the unified export script:

```bash
# Export all Polygon data types
./scripts/export_data.sh --network Polygon all

# Export just peer locations
./scripts/export_data.sh --network Polygon peers

# Export to custom directory
./scripts/export_data.sh --network Polygon -o ./exports/polygon all
```

This creates CSV files in `./exports/`:
- `peer_locations.csv` - All Polygon nodes with geolocation
- `peer_count_by_country.csv` - Nodes per country
- `peer_count_by_city.csv` - Nodes per city
- `peer_count_by_as.csv` - Nodes per Autonomous System
- `hosting_provider_distribution.csv` - Hosting provider analysis
- `client_distribution.csv` - Client software distribution

### Manual Export (Alternative)

If you prefer direct SQL queries:

```bash
psql -h localhost -U user -d armiarma -c "
  COPY (
    SELECT 
      p.peer_id,
      p.ip,
      p.port,
      p.user_agent,
      p.client_name,
      p.client_version,
      i.country,
      i.country_code,
      i.city,
      i.lat,
      i.lon,
      i.isp,
      i.org,
      i.asname,
      i.hosting
    FROM peer_info p
    LEFT JOIN ips i ON p.ip = i.ip
    WHERE p.network = 'Polygon'
      AND p.deprecated = false
    ORDER BY p.last_activity DESC
  ) TO '/tmp/polygon_validators.csv' CSV HEADER;
"
```

## Metrics

The Polygon crawler exposes Prometheus metrics on the configured metrics port (default: 9090):

- `discovery_*` - Discovery-related metrics
- `peering_*` - Peering strategy metrics  
- `host_*` - Host connection metrics
- `polygon_crawler_*` - Crawler-specific metrics

Access metrics at: `http://localhost:9090/metrics`

## Troubleshooting

### No peers discovered

- Check that bootnodes are reachable
- Verify firewall allows UDP on the configured port
- Check logs for discovery errors: `--log-level debug`

### Database connection errors

- Verify PostgreSQL is running
- Check connection string format
- Ensure database exists and user has permissions

### High memory usage

- Reduce `--peers-backup` interval
- Disable `--persist-connevents` if not needed
- Monitor with Prometheus metrics

## Implementation Details

### Bootnodes

The Polygon crawler uses the following default bootnodes:

```go
enode://b8f1cc9c5d4403703fbf377116469667d2b1823c0daf16b7250aa576bacf399e42c3930ccfcb02c5df6879565a2b8931335565f0e8d3f8e72385ecf4a4bf160a@3.36.224.80:30303
enode://8729e0c825f3d9cad382555f3e46dcff21af323e89025a0e6312df541f4a9e73abfa562d64906f5e59c51fe6f0501b3e61b07979606c56329c020ed739910759@54.194.245.5:30303
enode://681ebac58d8dd2d8a6eef15329dfbad0ab960561524cf2dfde40ad646736fe5c244020f20b87e7c1520820bc625cfb487dd71d63a3a3bf0bccb2b3b7e6c0b2c1@13.53.208.243:30303
```

### Discovery Protocol

The Polygon crawler uses discv4 (discovery protocol v4) which is compatible with Geth-based clients like Bor. This is different from Ethereum Consensus Layer which uses discv5.

### Network Type

All discovered Polygon nodes are tagged with network type `Polygon` in the database, allowing easy filtering and analysis.

## Integration with GeoBeat

Once the crawler has collected Polygon validator data, export it for geographic decentralization analysis:

```bash
# Export Polygon data directly to geobeat directory
./scripts/export_data.sh \
  --network Polygon \
  -o /path/to/geobeat/data/raw \
  all

# Or export to default location and copy
./scripts/export_data.sh --network Polygon all
cp ./exports/*.csv /path/to/geobeat/data/raw/

# Analyze with geobeat
cd /path/to/geobeat
# (Follow geobeat documentation for analysis)
```

## Contributing

When adding new features to the Polygon crawler:

1. Follow the existing pattern in `pkg/crawler/ethereum.go`
2. Add tests in `*_test.go` files
3. Update this documentation
4. Ensure backwards compatibility with database schema

## Support

For issues or questions:
- Check armiarma logs with `--log-level debug`
- Review database state with SQL queries
- Check Prometheus metrics for anomalies

