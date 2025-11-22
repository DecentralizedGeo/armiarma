package polygon

import (
	"github.com/migalabs/armiarma/pkg/utils"
)

// LocalPolygonNode represents a local node in the Polygon network
type LocalPolygonNode struct {
	chainID int
}

// NewLocalPolygonNode creates a new LocalPolygonNode
func NewLocalPolygonNode() *LocalPolygonNode {
	return &LocalPolygonNode{
		chainID: ChainID,
	}
}

// Network returns the network type for this node
func (n *LocalPolygonNode) Network() utils.NetworkType {
	return utils.PolygonNetwork
}

// ChainID returns the Polygon chain ID
func (n *LocalPolygonNode) ChainID() int {
	return n.chainID
}
