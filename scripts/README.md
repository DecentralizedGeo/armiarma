# Armiarma Data Export Scripts

## Overview

Scripts for exporting peer and geolocation data from the Armiarma PostgreSQL database.

## export_data.sh

**Unified export script** that supports all networks (Ethereum, Polygon, etc.)

### Features

- **Network Filtering**: Export data for specific networks or all networks
- **Multiple Export Types**: Peers, IPs, country stats, city stats, hosting, AS, clients
- **Flexible Configuration**: Use .env file or command-line options
- **Docker Support**: Automatically detects and uses Docker containers
- **Multiple Output Formats**: All exports are CSV with headers

### Quick Start

```bash
# Export all data for each network separately (default)
./scripts/export_data.sh

# Export only Polygon data
./scripts/export_data.sh --network Polygon

# Export all networks combined into single files
./scripts/export_data.sh --network ""
```

### Usage

```
./scripts/export_data.sh [OPTIONS] [EXPORT_TYPE]

EXPORT_TYPE:
    peers       Export active peer locations
    ips         Export all IPs with geolocation
    country     Export peer count by country
    city        Export peer count by city
    hosting     Export hosting provider distribution
    as          Export peer count by Autonomous System
    clients     Export client distribution
    all         Export all of the above (default)

OPTIONS:
    -n, --network NETWORK   Filter by network. Special values:
                            'all' - export for each network separately (default)
                            Specific network name (e.g., 'Polygon', 'Ethereum CL')
                            Empty string - combine all networks into single files
    -o, --output DIR        Output directory (default: ./exports)
    -H, --host HOST         Database host (overrides .env)
    -p, --port PORT         Database port (overrides .env)
    -u, --user USER         Database user (overrides .env)
    -d, --database DB       Database name (overrides .env)
    -c, --container NAME    Docker container name (overrides .env)
```

### Examples

#### Default: Export All Networks Separately

```bash
# Export all data for each network (creates polygon_*, ethereum_cl_*, etc.)
./scripts/export_data.sh

# Just peer locations for each network
./scripts/export_data.sh peers
```

#### Export Single Network

```bash
# All Polygon exports
./scripts/export_data.sh --network Polygon

# Just Polygon peer locations
./scripts/export_data.sh --network Polygon peers

# Polygon data to custom directory
./scripts/export_data.sh --network Polygon -o ./exports/polygon
```

#### Export All Networks Combined

```bash
# Combine all networks into single files (all_networks_* prefix)
./scripts/export_data.sh --network ""

# Just peer locations combined
./scripts/export_data.sh --network "" peers
```

### Output Files

When using `all` export type, the script creates (prefixed by network name):

- `{network}_peer_locations.csv` - Active peer locations with geolocation
- `{network}_all_ips.csv` - All IPs with geolocation data
- `{network}_peer_count_by_country.csv` - Node count per country
- `{network}_peer_count_by_city.csv` - Node count per city
- `{network}_hosting_provider_distribution.csv` - Hosting provider statistics
- `{network}_peer_count_by_as.csv` - Distribution by Autonomous System
- `{network}_client_distribution.csv` - Client software distribution

For example: `polygon_peer_locations.csv`, `ethereum_cl_peer_locations.csv`

### Configuration

The script loads configuration from `.env` file in the project root:

```bash
DB_HOST=localhost
DB_PORT=5432
DB_USER=user
DB_PASSWORD=password
DB_NAME=armiarmadb
CONTAINER_NAME=armiarma-db-1
OUTPUT_DIR=./exports
```

You can override any value with command-line options or environment variables.

### Docker Usage

The script automatically detects if you're running in Docker and uses the appropriate connection method:

```bash
# Using Docker container name from .env (exports all networks)
./scripts/export_data.sh

# Override container name
./scripts/export_data.sh -c my-postgres-container
```

### Network Names

Use these exact network names for filtering:

- `Polygon` - Polygon/Bor network
- `Ethereum CL` - Ethereum Consensus Layer
- `IPFS` - IPFS network
- `Filecoin` - Filecoin network

To see all available networks in your database:

```sql
SELECT DISTINCT network FROM peer_info;
```

### Integration with GeoBeat

Export data for GeoBeat analysis:

```bash
# Export all networks for geobeat
./scripts/export_data.sh -o /path/to/geobeat/data/raw

# Or export to default location and copy
./scripts/export_data.sh
cp ./exports/*.csv /path/to/geobeat/data/raw/
```

### Troubleshooting

#### No data exported

Check that the network name matches exactly:

```bash
# Check available networks in database
docker exec armiarma-db-1 psql -U user -d armiarmadb -c \
  "SELECT network, COUNT(*) FROM peer_info GROUP BY network;"
```

#### Connection errors

Ensure database is running:

```bash
# Check Docker container
docker ps | grep postgres

# Test direct connection
psql -h localhost -p 5432 -U user -d armiarmadb -c "SELECT 1;"
```

#### Permission errors

Make script executable:

```bash
chmod +x ./scripts/export_data.sh
```

## Best Practices

1. **Use network filters** for cleaner data analysis
2. **Export regularly** to capture network evolution
3. **Back up exports** before major changes
4. **Document export timestamps** for reproducibility
5. **Validate exports** by checking row counts

## Example Workflow

```bash
#!/bin/bash
# Daily export workflow

DATE=$(date +%Y%m%d)

# Export all networks separately (default behavior)
./scripts/export_data.sh -o ./exports/$DATE

# Or export specific networks only
./scripts/export_data.sh --network Polygon -o ./exports/polygon_$DATE
./scripts/export_data.sh --network "Ethereum CL" -o ./exports/ethereum_$DATE

echo "Exports complete for $DATE"
```

## Support

For issues or questions:
- Check script output for detailed error messages
- Verify database connection with manual psql test
- Review .env configuration
- Check Docker container status if using Docker
