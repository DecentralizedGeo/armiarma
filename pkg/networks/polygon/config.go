package polygon

import (
	"time"
)

const (
	// Polygon network identifiers
	PolygonMainnetChainID uint64 = 137
	PolygonMumbaiChainID  uint64 = 80001

	// Network names
	PolygonMainnetName string = "polygon-mainnet"
	PolygonMumbaiName  string = "polygon-mumbai"
)

var (
	// Genesis times for Polygon networks
	PolygonMainnetGenesis time.Time = time.Unix(1590824836, 0) // May 30, 2020
	PolygonMumbaiGenesis  time.Time = time.Unix(1558348800, 0) // May 20, 2019
)

// GetNetworkName returns the network name for the given chain ID
func GetNetworkName(chainID uint64) string {
	switch chainID {
	case PolygonMainnetChainID:
		return PolygonMainnetName
	case PolygonMumbaiChainID:
		return PolygonMumbaiName
	default:
		return "polygon-unknown"
	}
}

