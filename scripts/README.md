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
# Export all Polygon data
./scripts/export_data.sh --network Polygon all

# Export Ethereum peer locations
./scripts/export_data.sh --network "Ethereum CL" peers

# Export all networks (no filter)
./scripts/export_data.sh all
```

### Usage

```
./scripts/export_data.sh [OPTIONS] [EXPORT_TYPE]

EXPORT_TYPE:
    peers       Export active peer locations (default)
    ips         Export all IPs with geolocation
    country     Export peer count by country
    city        Export peer count by city
    hosting     Export hosting provider distribution
    as          Export peer count by Autonomous System
    clients     Export client distribution
    all         Export all of the above

OPTIONS:
    -n, --network NETWORK   Filter by network (e.g., 'Polygon', 'Ethereum CL')
                            If not specified, exports data for all networks
    -o, --output DIR        Output directory (default: ./exports)
    -H, --host HOST         Database host (overrides .env)
    -p, --port PORT         Database port (overrides .env)
    -u, --user USER         Database user (overrides .env)
    -d, --database DB       Database name (overrides .env)
    -c, --container NAME    Docker container name (overrides .env)
```

### Examples

#### Export Polygon Data

```bash
# All Polygon exports
./scripts/export_data.sh --network Polygon all

# Just Polygon peer locations
./scripts/export_data.sh --network Polygon peers

# Polygon data to custom directory
./scripts/export_data.sh --network Polygon -o ./exports/polygon all
```

#### Export Ethereum Data

```bash
# All Ethereum exports
./scripts/export_data.sh --network "Ethereum CL" all

# Just country distribution
./scripts/export_data.sh --network "Ethereum CL" country
```

#### Export All Networks

```bash
# Export all data from all networks
./scripts/export_data.sh all

# Just peer locations (all networks)
./scripts/export_data.sh peers
```

### Output Files

When using `all` export type, the script creates:

- `peer_locations.csv` - Active peer locations with geolocation
- `all_ips.csv` - All IPs with geolocation data
- `peer_count_by_country.csv` - Node count per country
- `peer_count_by_city.csv` - Node count per city
- `hosting_provider_distribution.csv` - Hosting provider statistics
- `peer_count_by_as.csv` - Distribution by Autonomous System
- `client_distribution.csv` - Client software distribution

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
# Using Docker container name from .env
./scripts/export_data.sh --network Polygon all

# Override container name
./scripts/export_data.sh -c my-postgres-container --network Polygon all
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
# Export Polygon data for geobeat
./scripts/export_data.sh --network Polygon -o /path/to/geobeat/data/raw all

# Or export to default location and copy
./scripts/export_data.sh --network Polygon all
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

# Export Polygon data
./scripts/export_data.sh \
  --network Polygon \
  -o ./exports/polygon_$DATE \
  all

# Export Ethereum data
./scripts/export_data.sh \
  --network "Ethereum CL" \
  -o ./exports/ethereum_$DATE \
  all

# Create combined export for comparison
./scripts/export_data.sh \
  -o ./exports/all_networks_$DATE \
  all

echo "Exports complete for $DATE"
```

## Support

For issues or questions:
- Check script output for detailed error messages
- Verify database connection with manual psql test
- Review .env configuration
- Check Docker container status if using Docker
