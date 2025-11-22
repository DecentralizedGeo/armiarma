package discv4

/**
This file implements the discovery4 service using the go-ethereum library
for discovering peers in Geth-based networks like Polygon Bor.
*/

import (
	"context"
	"crypto/ecdsa"
	"net"
	"sync"

	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"

	"github.com/migalabs/armiarma/pkg/db/models"
	"github.com/migalabs/armiarma/pkg/utils"

	gethlog "github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/p2p/discover"
	"github.com/ethereum/go-ethereum/p2p/enr"
	ethenode "github.com/ethereum/go-ethereum/p2p/enode"
)

var (
	ModuleName         = "DV4"
	NoNewPeerError     = errors.New("no new peer to read")
	ErrorNotValidNode  = errors.New("not valid node - different network")
)

type Discovery4 struct {
	// Service control variables
	ctx context.Context

	Dv4Listener *discover.UDPv4
	Iterator    ethenode.Iterator
	NetworkType utils.NetworkType

	// node notifier
	nodeNotC chan *models.HostInfo
	wg       sync.WaitGroup
	doneF    bool
}

// NewDiscovery4 creates a new Discovery4 service for Polygon/Bor network
func NewDiscovery4(
	ctx context.Context,
	privkey *ecdsa.PrivateKey,
	bootnodes []*ethenode.Node,
	port int,
	networkType utils.NetworkType) (*Discovery4, error) {

	log.Infof("launching discovery4 for network %s", networkType)

	if len(bootnodes) == 0 {
		log.Panic("unable to start dv4 peer discovery, no bootnodes provided")
	}

	// Create local node database (in-memory)
	localNode, err := createLocalNode(privkey, port)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create local node")
	}

	// UDP address to listen
	udpAddr := &net.UDPAddr{
		IP:   net.IPv4zero,
		Port: port,
	}

	// Start listening and create a connection object
	conn, err := net.ListenUDP("udp", udpAddr)
	if err != nil {
		return nil, errors.Wrap(err, "failed to listen on UDP")
	}

	// Set custom logger for the discovery4 service
	gethLogger := gethlog.New()

	// Configuration of the discovery4
	cfg := discover.Config{
		PrivateKey:  privkey,
		NetRestrict: nil,
		Bootnodes:   bootnodes,
		Log:         gethLogger,
	}

	// Start the discovery4 service and listen using the given connection
	dv4Listener, err := discover.ListenV4(conn, localNode, cfg)
	if err != nil {
		return nil, errors.Wrap(err, "failed to start discv4 listener")
	}

	// Return the Discovery object
	return &Discovery4{
		ctx:         ctx,
		Dv4Listener: dv4Listener,
		NetworkType: networkType,
		nodeNotC:    make(chan *models.HostInfo),
		doneF:       false,
	}, nil
}

// Start spawns the discovery service in a separate go-routine
func (d *Discovery4) Start() chan *models.HostInfo {
	// Generate the iterator over the found peers
	d.Iterator = d.Dv4Listener.RandomNodes()

	d.wg.Add(1)
	go d.nodeIterator()

	return d.nodeNotC
}

func (d *Discovery4) nodeIterator() {
	defer d.wg.Done()

	for {
		if d.doneF || d.ctx.Err() != nil {
			log.Info("shutdown detected, closing discv4 iterator")
			return
		}

		if d.Iterator.Next() {
			// Fill the given discovered peer with the next found peer
			node := d.Iterator.Node()
			log.WithFields(log.Fields{
				"node_id": node.ID().String(),
				"ip":      node.IP(),
				"tcp":     node.TCP(),
				"module":  "Discv4",
			}).Debug("new node discovered")

			hInfo, err := d.handleNode(node)
			if err != nil {
				if err != ErrorNotValidNode {
					log.Error(errors.Wrap(err, "error handling new node"))
				}
				continue
			}
			d.nodeNotC <- hInfo
		}
	}
}

// Stop closes the Discv4 node iterator properly
func (d *Discovery4) Stop() {
	d.doneF = true
	d.wg.Wait()

	d.Iterator.Close()
	d.Dv4Listener.Close()
	close(d.nodeNotC)
}

// handleNode parses and identifies all the advertised fields of a newly discovered peer
func (d *Discovery4) handleNode(node *ethenode.Node) (*models.HostInfo, error) {
	// Skip if node doesn't have TCP port (not connectable)
	if node.TCP() == 0 {
		return nil, errors.New("node has no TCP port")
	}

	// Skip if node doesn't have valid IP
	if node.IP() == nil {
		return nil, errors.New("node has no IP address")
	}

	// Convert node ID to libp2p peer ID
	peerID, err := utils.ConvertEnodeToPeerID(node)
	if err != nil {
		return nil, errors.Wrap(err, "unable to convert node ID to peer ID")
	}

	// Generate the HostInfo
	hInfo := models.NewHostInfo(
		peerID,
		d.NetworkType,
		models.WithIPAndPorts(
			node.IP().String(),
			node.TCP(),
		),
	)

	// Note: enode and node_id are not stored as attributes since they're not recognized by the DB
	// The peer ID and IP already contain the essential information

	return hInfo, nil
}

// createLocalNode creates a local enode.Node for the discv4 service
func createLocalNode(privkey *ecdsa.PrivateKey, port int) (*ethenode.LocalNode, error) {
	// Create in-memory database for the local node
	db, err := ethenode.OpenDB("")
	if err != nil {
		return nil, errors.Wrap(err, "failed to open node database")
	}

	localNode := ethenode.NewLocalNode(db, privkey)
	
	// Set the UDP and TCP endpoints
	localNode.SetStaticIP(net.ParseIP("0.0.0.0"))
	localNode.Set(enr.UDP(port))
	localNode.Set(enr.TCP(port))

	return localNode, nil
}

// ParseBootnodesFromStringSlice parses a slice of enode strings into enode.Node objects
func ParseBootnodesFromStringSlice(bNodes []string) []*ethenode.Node {
	bootNodeList := make([]*ethenode.Node, 0)

	for _, element := range bNodes {
		node, err := ethenode.Parse(ethenode.ValidSchemes, element)
		if err != nil {
			log.Warnf("failed to parse bootnode %s: %v", element, err)
			continue
		}
		bootNodeList = append(bootNodeList, node)
	}
	
	if len(bootNodeList) == 0 {
		log.Warn("no valid bootnodes parsed")
	}

	return bootNodeList
}
