#!/bin/bash
# Data Export Script for Armiarma
# Exports peer location and IP data from the PostgreSQL database

set -e

# Get the script's directory and project root
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

# Load environment variables from .env file if it exists
ENV_FILE_LOADED=false
if [ -f "$PROJECT_ROOT/.env" ]; then
    set -a  # automatically export all variables
    source "$PROJECT_ROOT/.env"
    set +a
    ENV_FILE_LOADED=true
else
    echo "Warning: .env file not found at $PROJECT_ROOT/.env"
    echo "Please create one based on .env_template or set environment variables manually."
fi

# Configuration (no hardcoded defaults)
DB_USER="${DB_USER}"
DB_PASSWORD="${DB_PASSWORD}"
DB_NAME="${DB_NAME}"
DB_HOST="${DB_HOST}"
DB_PORT="${DB_PORT}"
CONTAINER_NAME="${CONTAINER_NAME}"
OUTPUT_DIR="${OUTPUT_DIR:-$PROJECT_ROOT/exports}"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Helper functions
log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Validate required configuration
validate_config() {
    local missing_vars=()
    
    # Only validate DB credentials if we're not using Docker (which we'll detect later)
    # For now, just check that at least some config exists
    if [ -z "$DB_PORT" ] && [ -z "$CONTAINER_NAME" ]; then
        log_error "No database configuration found."
        log_error "Please ensure your .env file contains DB_PORT and/or CONTAINER_NAME"
        exit 1
    fi
}

# Create output directory if it doesn't exist
mkdir -p "$OUTPUT_DIR"

# Validate configuration
validate_config

# Function to check if we can connect directly to the database
can_connect_directly() {
    if [ -z "$DB_HOST" ] || [ -z "$DB_PORT" ] || [ -z "$DB_USER" ] || [ -z "$DB_NAME" ]; then
        return 1
    fi
    PGPASSWORD="$DB_PASSWORD" psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" -c '\q' 2>/dev/null
    return $?
}

# Function to check if docker container is running
is_docker_running() {
    if [ -z "$CONTAINER_NAME" ]; then
        # Try to find the container by searching for postgres containers
        CONTAINER_NAME=$(docker ps --filter "ancestor=postgres:latest" --format '{{.Names}}' | head -n 1)
        if [ -z "$CONTAINER_NAME" ]; then
            return 1
        fi
    fi
    docker ps --filter "name=$CONTAINER_NAME" --format '{{.Names}}' | grep -q "$CONTAINER_NAME"
    return $?
}

# Get database credentials for Docker (from docker-compose.yaml defaults)
get_docker_db_user() {
    echo "${DB_USER:-user}"
}

get_docker_db_name() {
    echo "${DB_NAME:-armiarmadb}"
}

# Export peer locations to CSV
export_peer_locations() {
    local output_file="$OUTPUT_DIR/${NETWORK_PREFIX}peer_locations.csv"
    log_info "Exporting peer locations to $output_file..."
    
    local query="COPY (
  SELECT 
    peer_info.peer_id,
    peer_info.network,
    peer_info.ip,
    peer_info.port,
    peer_info.user_agent,
    peer_info.client_name,
    peer_info.client_version,
    ips.country,
    ips.city,
    ips.lat,
    ips.lon,
    ips.isp,
    ips.org,
    ips.as_raw,
    ips.asname,
    ips.hosting
  FROM peer_info 
  INNER JOIN ips ON peer_info.ip = ips.ip
  WHERE peer_info.deprecated = false $NETWORK_WHERE
  ORDER BY peer_info.network, ips.country, ips.city
) TO STDOUT WITH CSV HEADER;"
    
    if can_connect_directly; then
        PGPASSWORD="$DB_PASSWORD" psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" -c "$query" > "$output_file"
    elif is_docker_running; then
        log_warn "Direct connection failed, using Docker exec..."
        local docker_user=$(get_docker_db_user)
        local docker_db=$(get_docker_db_name)
        docker exec -i "$CONTAINER_NAME" psql -U "$docker_user" -d "$docker_db" -c "$query" > "$output_file"
    else
        log_error "Cannot connect to database. Please check if the database is running."
        return 1
    fi
    
    local row_count=$(tail -n +2 "$output_file" | wc -l)
    log_info "Exported $row_count peer locations to $output_file"
}

# Export all IPs with geolocation to CSV
export_all_ips() {
    local output_file="$OUTPUT_DIR/${NETWORK_PREFIX}all_ips.csv"
    log_info "Exporting all IPs with geolocation to $output_file..."
    
    local query="COPY (
  SELECT DISTINCT
    ips.ip,
    ips.country,
    ips.country_code,
    ips.city,
    ips.region_name,
    ips.lat,
    ips.lon,
    ips.isp,
    ips.org,
    ips.as_raw,
    ips.asname,
    ips.hosting,
    ips.proxy,
    ips.mobile
  FROM ips
  INNER JOIN peer_info ON ips.ip = peer_info.ip
  WHERE peer_info.deprecated = false $NETWORK_WHERE
  ORDER BY ips.country, ips.city
) TO STDOUT WITH CSV HEADER;"
    
    if can_connect_directly; then
        PGPASSWORD="$DB_PASSWORD" psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" -c "$query" > "$output_file"
    elif is_docker_running; then
        log_warn "Direct connection failed, using Docker exec..."
        local docker_user=$(get_docker_db_user)
        local docker_db=$(get_docker_db_name)
        docker exec -i "$CONTAINER_NAME" psql -U "$docker_user" -d "$docker_db" -c "$query" > "$output_file"
    else
        log_error "Cannot connect to database. Please check if the database is running."
        return 1
    fi
    
    local row_count=$(tail -n +2 "$output_file" | wc -l)
    log_info "Exported $row_count IP records to $output_file"
}

# Export peer count by country
export_country_stats() {
    local output_file="$OUTPUT_DIR/${NETWORK_PREFIX}peer_count_by_country.csv"
    log_info "Exporting peer count by country to $output_file..."
    
    local query="COPY (
  SELECT 
    ips.country,
    ips.country_code,
    COUNT(DISTINCT peer_info.peer_id) AS peer_count,
    COUNT(DISTINCT ips.asname) as unique_asns,
    SUM(CASE WHEN ips.hosting THEN 1 ELSE 0 END) as hosted_nodes,
    ROUND(AVG(ips.lat)::numeric, 4) as avg_lat,
    ROUND(AVG(ips.lon)::numeric, 4) as avg_lon
  FROM peer_info
  INNER JOIN ips ON peer_info.ip = ips.ip
  WHERE peer_info.deprecated = false $NETWORK_WHERE
  GROUP BY ips.country, ips.country_code
  ORDER BY peer_count DESC
) TO STDOUT WITH CSV HEADER;"
    
    if can_connect_directly; then
        PGPASSWORD="$DB_PASSWORD" psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" -c "$query" > "$output_file"
    elif is_docker_running; then
        log_warn "Direct connection failed, using Docker exec..."
        local docker_user=$(get_docker_db_user)
        local docker_db=$(get_docker_db_name)
        docker exec -i "$CONTAINER_NAME" psql -U "$docker_user" -d "$docker_db" -c "$query" > "$output_file"
    else
        log_error "Cannot connect to database. Please check if the database is running."
        return 1
    fi
    
    local row_count=$(tail -n +2 "$output_file" | wc -l)
    log_info "Exported statistics for $row_count countries to $output_file"
}

# Export peer count by city
export_city_stats() {
    local output_file="$OUTPUT_DIR/${NETWORK_PREFIX}peer_count_by_city.csv"
    log_info "Exporting peer count by city to $output_file..."
    
    local query="COPY (
  SELECT 
    ips.city,
    ips.country,
    ips.country_code,
    COUNT(DISTINCT peer_info.peer_id) AS peer_count,
    ips.lat,
    ips.lon
  FROM peer_info
  INNER JOIN ips ON peer_info.ip = ips.ip
  WHERE peer_info.deprecated = false $NETWORK_WHERE
    AND ips.city != ''
  GROUP BY ips.city, ips.country, ips.country_code, ips.lat, ips.lon
  HAVING COUNT(*) >= 2
  ORDER BY peer_count DESC
) TO STDOUT WITH CSV HEADER;"
    
    if can_connect_directly; then
        PGPASSWORD="$DB_PASSWORD" psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" -c "$query" > "$output_file"
    elif is_docker_running; then
        log_warn "Direct connection failed, using Docker exec..."
        local docker_user=$(get_docker_db_user)
        local docker_db=$(get_docker_db_name)
        docker exec -i "$CONTAINER_NAME" psql -U "$docker_user" -d "$docker_db" -c "$query" > "$output_file"
    else
        log_error "Cannot connect to database. Please check if the database is running."
        return 1
    fi
    
    local row_count=$(tail -n +2 "$output_file" | wc -l)
    log_info "Exported statistics for $row_count cities to $output_file"
}

# Export hosting provider distribution
export_hosting_stats() {
    local output_file="$OUTPUT_DIR/${NETWORK_PREFIX}hosting_provider_distribution.csv"
    log_info "Exporting hosting provider distribution to $output_file..."
    
    local query="COPY (
  SELECT 
    ips.org as provider,
    COUNT(DISTINCT peer_info.peer_id) AS peer_count,
    COUNT(DISTINCT ips.country) as countries,
    ips.hosting
  FROM peer_info
  INNER JOIN ips ON peer_info.ip = ips.ip
  WHERE peer_info.deprecated = false $NETWORK_WHERE
    AND ips.org != ''
  GROUP BY ips.org, ips.hosting
  HAVING COUNT(*) >= 2
  ORDER BY peer_count DESC
) TO STDOUT WITH CSV HEADER;"
    
    if can_connect_directly; then
        PGPASSWORD="$DB_PASSWORD" psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" -c "$query" > "$output_file"
    elif is_docker_running; then
        log_warn "Direct connection failed, using Docker exec..."
        local docker_user=$(get_docker_db_user)
        local docker_db=$(get_docker_db_name)
        docker exec -i "$CONTAINER_NAME" psql -U "$docker_user" -d "$docker_db" -c "$query" > "$output_file"
    else
        log_error "Cannot connect to database. Please check if the database is running."
        return 1
    fi
    
    local row_count=$(tail -n +2 "$output_file" | wc -l)
    log_info "Exported statistics for $row_count hosting providers to $output_file"
}

# Export peer count by Autonomous System (AS)
export_as_stats() {
    local output_file="$OUTPUT_DIR/${NETWORK_PREFIX}peer_count_by_as.csv"
    log_info "Exporting peer count by Autonomous System to $output_file..."
    
    local query="COPY (
  SELECT 
    ips.asname,
    ips.as_raw,
    ips.org,
    COUNT(DISTINCT peer_info.peer_id) AS peer_count,
    COUNT(DISTINCT ips.country) as countries,
    SUM(CASE WHEN ips.hosting THEN 1 ELSE 0 END) as hosted_nodes
  FROM peer_info
  INNER JOIN ips ON peer_info.ip = ips.ip
  WHERE peer_info.deprecated = false $NETWORK_WHERE
    AND ips.asname != ''
  GROUP BY ips.asname, ips.as_raw, ips.org
  ORDER BY peer_count DESC
) TO STDOUT WITH CSV HEADER;"
    
    if can_connect_directly; then
        PGPASSWORD="$DB_PASSWORD" psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" -c "$query" > "$output_file"
    elif is_docker_running; then
        log_warn "Direct connection failed, using Docker exec..."
        local docker_user=$(get_docker_db_user)
        local docker_db=$(get_docker_db_name)
        docker exec -i "$CONTAINER_NAME" psql -U "$docker_user" -d "$docker_db" -c "$query" > "$output_file"
    else
        log_error "Cannot connect to database. Please check if the database is running."
        return 1
    fi
    
    local row_count=$(tail -n +2 "$output_file" | wc -l)
    log_info "Exported statistics for $row_count autonomous systems to $output_file"
}

# Export client distribution
export_client_stats() {
    local output_file="$OUTPUT_DIR/${NETWORK_PREFIX}client_distribution.csv"
    log_info "Exporting client distribution to $output_file..."
    
    local query="COPY (
  SELECT 
    client_name,
    client_version,
    client_os,
    client_arch,
    COUNT(*) as peer_count,
    ROUND(100.0 * COUNT(*) / SUM(COUNT(*)) OVER (), 2) as percentage
  FROM peer_info
  WHERE deprecated = false $NETWORK_WHERE
  GROUP BY client_name, client_version, client_os, client_arch
  ORDER BY peer_count DESC
) TO STDOUT WITH CSV HEADER;"
    
    if can_connect_directly; then
        PGPASSWORD="$DB_PASSWORD" psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" -c "$query" > "$output_file"
    elif is_docker_running; then
        log_warn "Direct connection failed, using Docker exec..."
        local docker_user=$(get_docker_db_user)
        local docker_db=$(get_docker_db_name)
        docker exec -i "$CONTAINER_NAME" psql -U "$docker_user" -d "$docker_db" -c "$query" > "$output_file"
    else
        log_error "Cannot connect to database. Please check if the database is running."
        return 1
    fi
    
    local row_count=$(tail -n +2 "$output_file" | wc -l)
    log_info "Exported statistics for $row_count client configurations to $output_file"
}

# Display usage information
usage() {
    cat << EOF
Usage: $0 [OPTIONS] [EXPORT_TYPE]

Export peer location and IP data from the Armiarma PostgreSQL database.

EXPORT_TYPE:
    peers       Export active peer locations (default)
    ips         Export all IPs with geolocation
    country     Export peer count by country
    city        Export peer count by city (top 50)
    hosting     Export hosting provider distribution (top 20)
    as          Export peer count by Autonomous System (top 50)
    clients     Export client distribution
    all         Export all of the above

OPTIONS:
    -h, --help              Show this help message
    -n, --network NETWORK   Filter by network (e.g., 'Polygon', 'Ethereum CL')
                            If not specified, exports data for all networks
    -o, --output DIR        Output directory (default: ./exports)
    -H, --host HOST         Database host (overrides .env)
    -p, --port PORT         Database port (overrides .env)
    -u, --user USER         Database user (overrides .env)
    -d, --database DB       Database name (overrides .env)
    -c, --container NAME    Docker container name (overrides .env)

CONFIGURATION:
    The script automatically loads variables from .env file in the project root.
    
    Supported environment variables:
        DB_HOST                 Database host
        DB_PORT                 Database port
        DB_USER                 Database user
        DB_PASSWORD             Database password
        DB_NAME                 Database name
        CONTAINER_NAME          Docker container name
        OUTPUT_DIR              Output directory

EXAMPLES:
    # Export peer locations (all networks)
    $0 peers

    # Export only Polygon data
    $0 --network Polygon all

    # Export only Ethereum data
    $0 --network "Ethereum CL" all

    # Export to a specific directory
    $0 -o /tmp/exports peers

    # Export Polygon data to custom directory
    $0 --network Polygon -o ./exports/polygon all

    # Override .env settings with environment variables
    DB_HOST=192.168.1.100 DB_PORT=5433 $0 peers
    
    # Override .env settings with command-line options
    $0 -H 192.168.1.100 -p 5433 peers

NOTE:
    The script loads database configuration from .env file.
    Ensure your .env file is properly configured before running.

EOF
}

# Parse command-line arguments
EXPORT_TYPE="peers"
NETWORK_FILTER=""

while [[ $# -gt 0 ]]; do
    case $1 in
        -h|--help)
            usage
            exit 0
            ;;
        -n|--network)
            NETWORK_FILTER="$2"
            shift 2
            ;;
        -o|--output)
            OUTPUT_DIR="$2"
            shift 2
            ;;
        -H|--host)
            DB_HOST="$2"
            shift 2
            ;;
        -p|--port)
            DB_PORT="$2"
            shift 2
            ;;
        -u|--user)
            DB_USER="$2"
            shift 2
            ;;
        -d|--database)
            DB_NAME="$2"
            shift 2
            ;;
        -c|--container)
            CONTAINER_NAME="$2"
            shift 2
            ;;
        peers|ips|country|city|hosting|as|clients|all)
            EXPORT_TYPE="$1"
            shift
            ;;
        *)
            log_error "Unknown option: $1"
            usage
            exit 1
            ;;
    esac
done

# Build WHERE clause for network filter and filename prefix
NETWORK_WHERE=""
NETWORK_PREFIX=""
if [ -n "$NETWORK_FILTER" ]; then
    NETWORK_WHERE="AND peer_info.network = '$NETWORK_FILTER'"
    # Convert network name to lowercase and replace spaces with underscores for filename
    NETWORK_PREFIX="$(echo "$NETWORK_FILTER" | tr '[:upper:]' '[:lower:]' | tr ' ' '_')_"
    log_info "Filtering by network: $NETWORK_FILTER"
else
    NETWORK_PREFIX="all_networks_"
fi

# Main execution
log_info "Starting data export..."
if [ "$ENV_FILE_LOADED" = true ]; then
    log_info "Loaded configuration from .env file"
fi
log_info "Database: $DB_HOST:$DB_PORT/$DB_NAME (user: $DB_USER)"
log_info "Output directory: $OUTPUT_DIR"

case $EXPORT_TYPE in
    peers)
        export_peer_locations
        ;;
    ips)
        export_all_ips
        ;;
    country)
        export_country_stats
        ;;
    city)
        export_city_stats
        ;;
    hosting)
        export_hosting_stats
        ;;
    as)
        export_as_stats
        ;;
    clients)
        export_client_stats
        ;;
    all)
        export_peer_locations
        export_all_ips
        export_country_stats
        export_city_stats
        export_hosting_stats
        export_as_stats
        export_client_stats
        ;;
    *)
        log_error "Unknown export type: $EXPORT_TYPE"
        usage
        exit 1
        ;;
esac

log_info "Export completed successfully!"
log_info "Files are available in: $OUTPUT_DIR"

