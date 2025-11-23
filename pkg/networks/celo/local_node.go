package celo

import (
	"github.com/migalabs/armiarma/pkg/utils"
)

// LocalCeloNode represents a local node in the Celo network
type LocalCeloNode struct {
	chainID int
}

// NewLocalCeloNode creates a new LocalCeloNode
func NewLocalCeloNode() *LocalCeloNode {
	return &LocalCeloNode{
		chainID: ChainID,
	}
}

// Network returns the network type for this node
func (n *LocalCeloNode) Network() utils.NetworkType {
	return utils.CeloNetwork
}

// ChainID returns the Celo chain ID
func (n *LocalCeloNode) ChainID() int {
	return n.chainID
}

