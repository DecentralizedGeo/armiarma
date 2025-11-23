package celo

import (
	"time"
)

const (
	// Celo network identifiers
	CeloMainnetChainID uint64 = 42220 // Celo L2 mainnet
	CeloSepoliaChainID uint64 = 44787 // Celo Sepolia testnet

	// Network names
	CeloMainnetName string = "celo-mainnet"
	CeloSepoliaName string = "celo-sepolia"
)

var (
	// Genesis times for Celo networks
	// March 26, 2025 3:00 AM UTC - L2 migration date from docs
	CeloMainnetL2Genesis time.Time = time.Unix(1743051600, 0)
	CeloSepoliaGenesis   time.Time = time.Unix(1700000000, 0) // Placeholder
)

// GetNetworkName returns the network name for the given chain ID
func GetNetworkName(chainID uint64) string {
	switch chainID {
	case CeloMainnetChainID:
		return CeloMainnetName
	case CeloSepoliaChainID:
		return CeloSepoliaName
	default:
		return "celo-unknown"
	}
}

