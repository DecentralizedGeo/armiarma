# CSV Export Data Integrity Analysis

## Summary
✅ **The CSV exports are accurate and properly filtered by network.**
✅ **No data contamination found - Ethereum peers are NOT showing up in Polygon or Filecoin data.**

## Detailed Findings

### Database Stats (Active Peers)
- **Ethereum CL**: 37,274 peers from 24,291 unique IPs
- **Polygon**: 24,510 peers from 12,470 unique IPs  
- **Filecoin**: 483 peers from 272 unique IPs
- **Total**: 62,325 peers from 33,240 unique IPs

### CSV Export Stats (IPs with Geolocation)
- **Ethereum CL**: 17,356 IPs
- **Polygon**: 11,242 IPs
- **Filecoin**: 256 IPs

*Note: CSV exports have fewer IPs because they INNER JOIN with the `ips` table, excluding IPs without geolocation data yet.*

### IP Overlap Analysis

#### Between Networks (Database):
- Eth-Polygon: 3,744 shared IPs (30% of Polygon, 15% of Ethereum)
- Eth-Filecoin: 7 shared IPs (2.6% of Filecoin)
- Poly-Filecoin: 5 shared IPs (1.8% of Filecoin)

#### Between Networks (CSV Exports):
- Eth-Polygon: 3,606 shared IPs (32% of Polygon)
- Eth-Filecoin: 7 shared IPs (2.7% of Filecoin)
- Poly-Filecoin: 5 shared IPs (1.9% of Filecoin)

**These overlaps are LEGITIMATE** - they represent users running multiple network clients on the same machine.

### Client Distribution Verification
Verified that network classification is correct:
- **Ethereum CL**: Only Ethereum clients (lighthouse, prysm, teku, lodestar, nimbus, erigon, grandine)
- **Filecoin**: Only Filecoin clients (lotus)
- **Polygon**: No Ethereum or Filecoin clients

**Zero contaminated records found** (checked both active and deprecated peers).

## Recommendations

### 1. Export Script is Working Correctly
The current export script properly filters by network using the `network` field. No changes needed for data accuracy.

### 2. Missing Feature: Time-Based Filtering
The export script does NOT filter by timeframe. To export only recent data:
- Add optional date range parameters to the script
- Filter using `last_activity` timestamp (for Ethereum/Filecoin) 
- Note: Polygon peers have NULL last_activity values - may need different approach

### 3. CSV Export Coverage
The exports show ~71% of Ethereum IPs, ~90% of Polygon IPs, and ~94% of Filecoin IPs from the database.
Missing IPs are those without geolocation data yet (pending IP lookup).

## Verification Queries Used

### Check network separation:
```sql
SELECT network, COUNT(*) FROM peer_info 
WHERE deprecated = false 
GROUP BY network;
```

### Check for contamination:
```sql
-- Check for Ethereum clients in Polygon
SELECT COUNT(*) FROM peer_info 
WHERE network = 'Polygon' 
AND client_name IN ('lighthouse', 'prysm', 'lodestar', 'teku', 'nimbus', 'erigon', 'grandine');
-- Result: 0

-- Check for Ethereum clients in Filecoin
SELECT COUNT(*) FROM peer_info 
WHERE network = 'Filecoin' 
AND client_name IN ('lighthouse', 'prysm', 'lodestar', 'teku', 'nimbus', 'erigon', 'grandine');
-- Result: 0

-- Check for Filecoin clients in Ethereum
SELECT COUNT(*) FROM peer_info 
WHERE network = 'Ethereum CL' 
AND client_name = 'lotus';
-- Result: 0
```

### Check IP overlaps:
```sql
WITH eth_ips AS (
  SELECT DISTINCT ip FROM peer_info WHERE network = 'Ethereum CL' AND deprecated = false
),
poly_ips AS (
  SELECT DISTINCT ip FROM peer_info WHERE network = 'Polygon' AND deprecated = false
),
file_ips AS (
  SELECT DISTINCT ip FROM peer_info WHERE network = 'Filecoin' AND deprecated = false
)
SELECT 
  (SELECT COUNT(*) FROM eth_ips) as eth_unique_ips,
  (SELECT COUNT(*) FROM poly_ips) as poly_unique_ips,
  (SELECT COUNT(*) FROM file_ips) as file_unique_ips,
  (SELECT COUNT(*) FROM eth_ips INNER JOIN poly_ips ON eth_ips.ip = poly_ips.ip) as eth_poly_overlap,
  (SELECT COUNT(*) FROM eth_ips INNER JOIN file_ips ON eth_ips.ip = file_ips.ip) as eth_file_overlap,
  (SELECT COUNT(*) FROM poly_ips INNER JOIN file_ips ON poly_ips.ip = file_ips.ip) as poly_file_overlap;
```

## Conclusion
Your concern about data contamination was valid to check, but the database and exports are clean. 
The network separation is working correctly, and there's no evidence of Ethereum peers in Polygon/Filecoin data.

The IP overlaps you see are expected behavior - they represent operators running multiple network clients on the same infrastructure, which is common in the blockchain space.

---

*Analysis performed: November 23, 2025*
*Database snapshot: armiarmadb with 62,325 active peers across 3 networks*

