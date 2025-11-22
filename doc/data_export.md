# Data Export Guide

This document describes how to export peer location and IP data from the Armiarma database in various formats.

## Quick Start: Using the Export Script

The easiest way to export data is using the provided export script:

```bash
# Export peer locations (default)
./scripts/export_data.sh

# Export all data types
./scripts/export_data.sh all

# Export specific data types
./scripts/export_data.sh peers       # Active peer locations
./scripts/export_data.sh ips         # All IPs with geolocation
./scripts/export_data.sh country     # Peer count by country
./scripts/export_data.sh city        # Peer count by city
./scripts/export_data.sh hosting     # Hosting provider distribution

# Specify output directory
./scripts/export_data.sh -o /path/to/output peers

# View all options
./scripts/export_data.sh --help
```

The script automatically:
- Loads configuration from your `.env` file (including `DB_PORT` and other variables)
- Has no hardcoded defaults - relies on your project's `.env` configuration
- Handles connection to the database (either directly or via Docker)
- Exports data to CSV files in the `./exports` directory by default

## Manual Export Methods

If you prefer to run the SQL queries manually, continue reading below.

## Prerequisites

You need access to the PostgreSQL database. You can connect using:

```bash
# From the host machine (if port is exposed)
psql -h localhost -p <DB_PORT> -U user -d armiarmadb

# From within the Docker network
docker exec -it <container_name> psql -U user -d armiarmadb
```

## CSV Export

### Export Active Peer Locations to CSV

This query exports peer IDs, IP addresses, and geolocation data for all non-deprecated peers:

```sql
COPY (
  SELECT 
    peer_info.peer_id,
    peer_info.ip,
    peer_info.port,
    ips.country,
    ips.city,
    ips.lat,
    ips.lon,
    ips.isp,
    ips.org,
    ips.hosting
  FROM peer_info 
  INNER JOIN ips ON peer_info.ip = ips.ip
  WHERE peer_info.deprecated = false
  ORDER BY ips.country, ips.city
) TO '/tmp/peer_locations.csv' WITH CSV HEADER;
```

**Note:** The output file path must be writable by the PostgreSQL server process. If running in Docker, you may need to use a path inside the container or mount a volume.

### Export All Discovered IPs with Geolocation

To export all unique IP addresses with their geolocation data:

```sql
COPY (
  SELECT 
    ip,
    country,
    country_code,
    city,
    region_name,
    lat,
    lon,
    isp,
    org,
    asname,
    hosting,
    proxy,
    mobile
  FROM ips
  ORDER BY country, city
) TO '/tmp/all_ips.csv' WITH CSV HEADER;
```

## JSON Export

### Export Active Peer Locations to JSON

Use psql's output redirection to create a JSON file:

```sql
\o /tmp/peer_locations.json
SELECT json_agg(row_to_json(t)) FROM (
  SELECT 
    peer_info.peer_id,
    peer_info.ip,
    peer_info.port,
    ips.country,
    ips.city,
    ips.lat,
    ips.lon,
    ips.isp,
    ips.org,
    ips.hosting
  FROM peer_info 
  INNER JOIN ips ON peer_info.ip = ips.ip
  WHERE peer_info.deprecated = false
  ORDER BY ips.country, ips.city
) t;
\o
```

The `\o` command redirects output to a file. The second `\o` with no argument resets output to stdout.

### Export with Pretty Formatting

For more readable JSON output:

```sql
\o /tmp/peer_locations_pretty.json
SELECT json_agg(row_to_json(t), true) FROM (
  SELECT 
    peer_info.peer_id,
    peer_info.ip,
    peer_info.port,
    ips.country,
    ips.city,
    ips.lat,
    ips.lon,
    ips.isp,
    ips.org,
    ips.hosting
  FROM peer_info 
  INNER JOIN ips ON peer_info.ip = ips.ip
  WHERE peer_info.deprecated = false
  ORDER BY ips.country, ips.city
) t;
\o
```

## GeoJSON Export

For GIS applications and mapping tools, export in GeoJSON format:

```sql
\o /tmp/peer_locations.geojson
SELECT json_build_object(
  'type', 'FeatureCollection',
  'features', json_agg(
    json_build_object(
      'type', 'Feature',
      'geometry', json_build_object(
        'type', 'Point',
        'coordinates', json_build_array(ips.lon, ips.lat)
      ),
      'properties', json_build_object(
        'peer_id', peer_info.peer_id,
        'ip', peer_info.ip,
        'country', ips.country,
        'city', ips.city,
        'isp', ips.isp,
        'hosting', ips.hosting
      )
    )
  )
)
FROM peer_info
INNER JOIN ips ON peer_info.ip = ips.ip
WHERE peer_info.deprecated = false;
\o
```

## Statistics Queries

### Peer Count by Country

```sql
SELECT 
  ips.country,
  ips.country_code,
  COUNT(DISTINCT peer_info.peer_id) AS peer_count
FROM peer_info
INNER JOIN ips ON peer_info.ip = ips.ip
WHERE peer_info.deprecated = false
GROUP BY ips.country, ips.country_code
ORDER BY peer_count DESC;
```

### Peer Count by City

```sql
SELECT 
  ips.country,
  ips.city,
  COUNT(DISTINCT peer_info.peer_id) AS peer_count
FROM peer_info
INNER JOIN ips ON peer_info.ip = ips.ip
WHERE peer_info.deprecated = false
GROUP BY ips.country, ips.city
ORDER BY peer_count DESC
LIMIT 50;
```

### Hosting Provider Distribution

```sql
SELECT 
  ips.org,
  ips.hosting,
  COUNT(DISTINCT peer_info.peer_id) AS peer_count
FROM peer_info
INNER JOIN ips ON peer_info.ip = ips.ip
WHERE peer_info.deprecated = false AND ips.hosting = true
GROUP BY ips.org, ips.hosting
ORDER BY peer_count DESC
LIMIT 20;
```

## Grafana Export

You can also export data directly from Grafana:

1. Navigate to the "Geographic Distribution" panel in the Armiarma Monitor dashboard
2. Click the panel title and select "Inspect" → "Data"
3. Click "Download CSV" or "Download Excel" to export the visible data

## Alternative: Using psql with Docker

If you need to export from within a Docker container:

```bash
# Enter the database container
docker exec -it armiarma-db-1 bash

# Inside the container, run psql
psql -U user -d armiarmadb

# Then run any of the SQL queries above
```

For direct command execution:

```bash
# Export CSV directly
docker exec -it armiarma-db-1 psql -U user -d armiarmadb -c "\
COPY (
  SELECT peer_id, peer_info.ip, port, country, city, lat, lon
  FROM peer_info 
  INNER JOIN ips ON peer_info.ip = ips.ip
  WHERE deprecated = false
) TO STDOUT WITH CSV HEADER;" > peer_locations.csv

# Export JSON directly
docker exec -it armiarma-db-1 psql -U user -d armiarmadb -t -c "\
SELECT json_agg(row_to_json(t)) FROM (
  SELECT peer_id, peer_info.ip, port, country, city, lat, lon
  FROM peer_info 
  INNER JOIN ips ON peer_info.ip = ips.ip
  WHERE deprecated = false
) t;" > peer_locations.json
```

## Notes

- All exports include only non-deprecated peers by default (where `deprecated = false`)
- Geolocation data is cached for 30 days, as configured in the IP locator service
- The IP geolocation service used is [ip-api.com](http://ip-api.com) (free tier, 45 requests/minute)
- Latitude and longitude coordinates are stored as `REAL` (floating-point) values
- For large datasets, consider adding pagination with `LIMIT` and `OFFSET` clauses

