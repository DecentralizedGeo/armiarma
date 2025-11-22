# Armiarma Scripts

This directory contains utility scripts for the Armiarma project.

## export_data.sh

Export peer location and IP data from the PostgreSQL database to CSV format.

### Usage

```bash
# Export peer locations (default)
./scripts/export_data.sh

# Export all data types
./scripts/export_data.sh all

# View help
./scripts/export_data.sh --help
```

### Features

- Automatically detects database connection method (direct or via Docker)
- Supports multiple export types (peers, IPs, statistics)
- Configurable via environment variables or command-line options
- Color-coded output for easy monitoring
- Creates exports directory automatically

### Export Types

- `peers` - Active peer locations with geolocation data
- `ips` - All discovered IPs with complete geolocation information
- `country` - Peer count statistics by country
- `city` - Peer count statistics by city (top 50)
- `hosting` - Hosting provider distribution (top 20)
- `all` - Export all of the above

### Configuration

The script automatically loads variables from the `.env` file in the project root if it exists. This is the recommended way to configure the script.

**Using .env file (recommended):**

The script will read `DB_PORT` and other variables from your existing `.env` file. No additional configuration needed!

**Using environment variables:**

You can override settings by setting environment variables:

```bash
DB_HOST=localhost DB_PORT=5433 ./scripts/export_data.sh peers
```

**Using command-line options:**

```bash
./scripts/export_data.sh -o /tmp/exports -H localhost -p 5433 peers
```

**Configuration precedence:**

Command-line options > Environment variables > .env file > Defaults

**Available variables (loaded from .env):**

```bash
DB_HOST                   # Database host
DB_PORT                   # Database port (from .env)
DB_USER                   # Database user
DB_PASSWORD               # Database password
DB_NAME                   # Database name
CONTAINER_NAME            # Docker container name
OUTPUT_DIR                # Output directory (defaults to ./exports)
```

**Note:** The script does not have hardcoded defaults for database credentials. It relies on your `.env` file configuration. For Docker deployments, it will use the values from `docker-compose.yaml` (user/armiarmadb) if not specified.

See `./scripts/export_data.sh --help` for full documentation.

