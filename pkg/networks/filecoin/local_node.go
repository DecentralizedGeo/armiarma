/*
Copyright © 2021 Miga Labs
*/
package filecoin

import (
	"github.com/migalabs/armiarma/pkg/utils"
)

// LocalFilecoinNode represents a local node in the Filecoin network
type LocalFilecoinNode struct{}

// NewLocalFilecoinNode creates a new LocalFilecoinNode
func NewLocalFilecoinNode() *LocalFilecoinNode {
	return &LocalFilecoinNode{}
}

// Network returns the network type for this node
func (n *LocalFilecoinNode) Network() utils.NetworkType {
	return utils.FilecoinNetwork
}
