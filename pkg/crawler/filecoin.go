/*
Copyright © 2021 Miga Labs
*/
package crawler

import (
	"context"
	"time"

	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"
	ma "github.com/multiformats/go-multiaddr"
	"github.com/prometheus/client_golang/prometheus"
	cli "github.com/urfave/cli/v2"

	"github.com/migalabs/armiarma/pkg/config"
	psql "github.com/migalabs/armiarma/pkg/db/postgresql"
	"github.com/migalabs/armiarma/pkg/discovery"
	"github.com/migalabs/armiarma/pkg/discovery/kdht"
	"github.com/migalabs/armiarma/pkg/hosts"
	"github.com/migalabs/armiarma/pkg/metrics"
	"github.com/migalabs/armiarma/pkg/networks/filecoin"
	"github.com/migalabs/armiarma/pkg/peering"
	"github.com/migalabs/armiarma/pkg/utils"
	"github.com/migalabs/armiarma/pkg/utils/apis"
	log "github.com/sirupsen/logrus"
)

var (
	filecoinDiscoveryTimeout = 30 * time.Second
)

// FilecoinCrawler is the crawler for the Filecoin network
type FilecoinCrawler struct {
	ctx       context.Context
	cancel    context.CancelFunc
	Host      *hosts.BasicLibp2pHost
	DB        *psql.DBClient
	Disc      *discovery.Discovery
	Peering   peering.PeeringService
	IpLocator *apis.IpLocator
	Metrics   *metrics.PrometheusMetrics
}

func NewFilecoinCrawler(mainCtx *cli.Context, conf config.FilecoinCrawlerConfig) (*FilecoinCrawler, error) {
	// Setup the configuration
	log.SetLevel(utils.ParseLogLevel(conf.LogLevel))

	ctx, cancel := context.WithCancel(mainCtx.Context)
	var err error

	// Parse or create a private key for the host
	var libp2pPrivKey crypto.PrivKey
	if conf.PrivateKey == "" {
		libp2pPrivKey, _, err = crypto.GenerateKeyPair(crypto.Secp256k1, 256)
		if err != nil {
			cancel()
			return nil, err
		}
	} else {
		gethPrivKey, err := utils.ParseECDSAPrivateKey(conf.PrivateKey)
		if err != nil {
			cancel()
			return nil, err
		}
		libp2pPrivKey, err = utils.AdaptSecp256k1FromECDSA(gethPrivKey)
		if err != nil {
			cancel()
			return nil, err
		}
	}

	// Generate the central metrics service
	promethMetrics := metrics.NewPrometheusMetrics(ctx, conf.MetricsIP, conf.MetricsPort)

	// Generate/connect to PSQL Database
	backupInterval, err := time.ParseDuration(conf.ActivePeersBackupInterval)
	if err != nil {
		cancel()
		return nil, err
	}
	dbClient, err := psql.NewDBClient(
		ctx,
		conf.Network(),
		conf.PsqlEndpoint,
		backupInterval,
		psql.InitializeTables(true),
		psql.WithConnectionEventsPersist(conf.PersistConnEvents),
	)
	if err != nil {
		cancel()
		return nil, err
	}

	// Create an IP locator instance
	ipLocator := apis.NewIpLocator(ctx, dbClient)

	// Create a local Filecoin node
	filecoinNode := filecoin.NewLocalFilecoinNode()

	// Generate libp2p host
	host, err := hosts.NewBasicLibp2pEth2Host(
		ctx,
		conf.IP,
		conf.Port,
		libp2pPrivKey,
		conf.UserAgent,
		filecoinNode,
		ipLocator,
	)
	if err != nil {
		cancel()
		return nil, err
	}

	// Parse bootnodes from multiaddr strings
	bootnodes, err := parseFilecoinBootnodes(conf.Bootnodes)
	if err != nil {
		cancel()
		return nil, err
	}

	// Create a new KadDHT discovery service
	kdhtDisc := kdht.NewKadDHTDiscService(
		ctx,
		host.Host(),
		conf.Network(),
		config.Filecoinprotocols,
		bootnodes,
		filecoinDiscoveryTimeout,
	)

	disc := discovery.NewDiscovery(
		ctx,
		kdhtDisc,
		dbClient,
		ipLocator,
	)

	// Generate the peering strategy
	pStrategy, err := peering.NewPruningStrategy(
		ctx,
		conf.Network(),
		dbClient,
	)
	if err != nil {
		cancel()
		return nil, err
	}

	// Generate the PeeringService
	peeringServ, err := peering.NewPeeringService(
		ctx,
		host,
		dbClient,
		peering.WithPeeringStrategy(pStrategy),
	)
	if err != nil {
		cancel()
		return nil, err
	}

	// Generate the FilecoinCrawler
	crawler := &FilecoinCrawler{
		ctx:       ctx,
		cancel:    cancel,
		Host:      host,
		DB:        dbClient,
		Disc:      disc,
		Peering:   peeringServ,
		IpLocator: ipLocator,
		Metrics:   promethMetrics,
	}

	// Register the metrics for the crawler and submodules
	crawlMetricsMod := crawler.GetMetrics()
	promethMetrics.AddMeticsModule(crawlMetricsMod)

	pruneMetricsMod := peeringServ.GetMetrics()
	promethMetrics.AddMeticsModule(pruneMetricsMod)

	hostMetricsMod := host.GetMetrics()
	promethMetrics.AddMeticsModule(hostMetricsMod)

	return crawler, nil
}

// parseFilecoinBootnodes converts multiaddr strings to peer.AddrInfo
func parseFilecoinBootnodes(bootnodeStrs []string) ([]peer.AddrInfo, error) {
	bootnodes := make([]peer.AddrInfo, 0, len(bootnodeStrs))
	for _, bnStr := range bootnodeStrs {
		maddr, err := ma.NewMultiaddr(bnStr)
		if err != nil {
			log.Warnf("failed to parse bootnode multiaddr %s: %s", bnStr, err)
			continue
		}
		peerInfo, err := peer.AddrInfoFromP2pAddr(maddr)
		if err != nil {
			log.Warnf("failed to extract peer info from %s: %s", bnStr, err)
			continue
		}
		bootnodes = append(bootnodes, *peerInfo)
	}
	if len(bootnodes) == 0 {
		log.Warn("no valid bootnodes parsed")
	}
	return bootnodes, nil
}

// Run starts all the crawler subroutines
func (c *FilecoinCrawler) Run() {
	// Initialization sequence for the crawler
	c.IpLocator.Run()
	c.Host.Start()
	c.Disc.Start()
	c.Peering.Run()
	c.Metrics.Start()
}

// Close shuts down the crawler gracefully
func (c *FilecoinCrawler) Close() {
	c.Disc.Stop()
	c.Host.Host().Close()
	c.DB.Close()
	c.Metrics.Close()
	c.cancel()
}

// GetMetrics returns the metrics module for the crawler
func (c *FilecoinCrawler) GetMetrics() *metrics.MetricsModule {
	metricsMod := metrics.NewMetricsModule(
		"crawler",
		"Filecoin Crawler metrics",
	)
	// compose all the metrics
	metricsMod.AddIndvMetric(c.clientDistributionMetrics())
	metricsMod.AddIndvMetric(c.versionDistributionMetrics())
	metricsMod.AddIndvMetric(c.geoDistributionMetrics())
	metricsMod.AddIndvMetric(c.nodeDistributionMetrics())
	metricsMod.AddIndvMetric(c.deprecatedNodeMetrics())
	metricsMod.AddIndvMetric(c.getPeersOs())
	metricsMod.AddIndvMetric(c.getPeersArch())
	metricsMod.AddIndvMetric(c.getHostedPeers())
	metricsMod.AddIndvMetric(c.getRTTDist())
	metricsMod.AddIndvMetric(c.getIPDist())

	return metricsMod
}

func (c *FilecoinCrawler) clientDistributionMetrics() *metrics.IndvMetrics {
	initFn := func() error {
		prometheus.MustRegister(ClientDistribution)
		return nil
	}
	updateFn := func() (interface{}, error) {
		summary, err := c.DB.GetClientDistribution()
		if err != nil {
			return nil, err
		}
		for cliName, cnt := range summary {
			ClientDistribution.WithLabelValues(cliName).Set(float64(cnt.(int)))
		}
		return summary, nil
	}
	cliDist, err := metrics.NewIndvMetrics(
		"client_distribution",
		initFn,
		updateFn,
	)
	if err != nil {
		return nil
	}
	return cliDist
}

func (c *FilecoinCrawler) versionDistributionMetrics() *metrics.IndvMetrics {
	initFn := func() error {
		prometheus.MustRegister(VersionDistribution)
		return nil
	}
	updateFn := func() (interface{}, error) {
		summary, err := c.DB.GetVersionDistribution()
		if err != nil {
			return nil, err
		}
		for cliVer, cnt := range summary {
			VersionDistribution.WithLabelValues(cliVer).Set(float64(cnt.(int)))
		}
		return summary, nil
	}
	versDist, err := metrics.NewIndvMetrics(
		"client_version_distribution",
		initFn,
		updateFn,
	)
	if err != nil {
		return nil
	}
	return versDist
}

func (c *FilecoinCrawler) geoDistributionMetrics() *metrics.IndvMetrics {
	initFn := func() error {
		prometheus.MustRegister(GeoDistribution)
		return nil
	}
	updateFn := func() (interface{}, error) {
		summary, err := c.DB.GetGeoDistribution()
		if err != nil {
			return nil, err
		}
		for country, cnt := range summary {
			GeoDistribution.WithLabelValues(country).Set(float64(cnt.(int)))
		}
		return summary, nil
	}
	versDist, err := metrics.NewIndvMetrics(
		"geographical_distribution",
		initFn,
		updateFn,
	)
	if err != nil {
		return nil
	}
	return versDist
}

func (c *FilecoinCrawler) nodeDistributionMetrics() *metrics.IndvMetrics {
	initFn := func() error {
		prometheus.MustRegister(NodeDistribution)
		return nil
	}
	updateFn := func() (interface{}, error) {
		peerLs, err := c.DB.GetNonDeprecatedPeers()
		if err != nil {
			return nil, err
		}

		NodeDistribution.Set(float64(len(peerLs)))

		return len(peerLs), nil
	}
	nodeDist, err := metrics.NewIndvMetrics(
		"geographical_distribution",
		initFn,
		updateFn,
	)
	if err != nil {
		return nil
	}
	return nodeDist
}

func (c *FilecoinCrawler) deprecatedNodeMetrics() *metrics.IndvMetrics {
	initFn := func() error {
		prometheus.MustRegister(DeprecatedCount)
		return nil
	}
	updateFn := func() (interface{}, error) {
		nodeCnt, err := c.DB.GetDeprecatedNodes()
		if err != nil {
			return nil, err
		}
		DeprecatedCount.Set(float64(nodeCnt))
		return nodeCnt, nil
	}
	depNodes, err := metrics.NewIndvMetrics(
		"deprecated_nodes",
		initFn,
		updateFn,
	)
	if err != nil {
		return nil
	}
	return depNodes
}

func (c *FilecoinCrawler) getPeersOs() *metrics.IndvMetrics {
	initFn := func() error {
		prometheus.MustRegister(OsDistribution)
		return nil
	}
	updateFn := func() (interface{}, error) {
		osDist, err := c.DB.GetOsDistribution()
		if err != nil {
			return nil, err
		}
		for key, val := range osDist {
			OsDistribution.WithLabelValues(key).Set(float64(val.(int)))
		}
		return osDist, nil
	}
	osMetr, err := metrics.NewIndvMetrics(
		"os_distribution",
		initFn,
		updateFn,
	)
	if err != nil {
		return nil
	}
	return osMetr
}

func (c *FilecoinCrawler) getPeersArch() *metrics.IndvMetrics {
	initFn := func() error {
		prometheus.MustRegister(ArchDistribution)
		return nil
	}
	updateFn := func() (interface{}, error) {
		archDist, err := c.DB.GetArchDistribution()
		if err != nil {
			return nil, err
		}
		for key, val := range archDist {
			ArchDistribution.WithLabelValues(key).Set(float64(val.(int)))
		}
		return archDist, nil
	}
	archMetr, err := metrics.NewIndvMetrics(
		"arch_distribution",
		initFn,
		updateFn,
	)
	if err != nil {
		return nil
	}
	return archMetr
}

func (c *FilecoinCrawler) getHostedPeers() *metrics.IndvMetrics {
	initFn := func() error {
		prometheus.MustRegister(HostedPeers)
		return nil
	}
	updateFn := func() (interface{}, error) {
		ipSummary, err := c.DB.GetHostingDistribution()
		if err != nil {
			return nil, err
		}
		for key, val := range ipSummary {
			HostedPeers.WithLabelValues(key).Set(float64(val.(int)))
		}
		return ipSummary, nil
	}
	ipHosting, err := metrics.NewIndvMetrics(
		"hosted_peer_distribution",
		initFn,
		updateFn,
	)
	if err != nil {
		return nil
	}
	return ipHosting
}

func (c *FilecoinCrawler) getRTTDist() *metrics.IndvMetrics {
	initFn := func() error {
		prometheus.MustRegister(RttDist)
		return nil
	}
	updateFn := func() (interface{}, error) {
		summary, err := c.DB.GetRTTDistribution()
		if err != nil {
			return nil, err
		}
		for key, val := range summary {
			RttDist.WithLabelValues(key).Set(float64(val.(int)))
		}
		return summary, nil
	}
	indvMetric, err := metrics.NewIndvMetrics(
		"rtt_distribution",
		initFn,
		updateFn,
	)
	if err != nil {
		return nil
	}
	return indvMetric
}

func (c *FilecoinCrawler) getIPDist() *metrics.IndvMetrics {
	initFn := func() error {
		prometheus.MustRegister(IPDist)
		return nil
	}
	updateFn := func() (interface{}, error) {
		summary, err := c.DB.GetIPDistribution()
		if err != nil {
			return nil, err
		}
		for key, val := range summary {
			IPDist.WithLabelValues(key).Set(float64(val.(int)))
		}
		return summary, nil
	}
	indvMetric, err := metrics.NewIndvMetrics(
		"ip_distribution",
		initFn,
		updateFn,
	)
	if err != nil {
		return nil
	}
	return indvMetric
}
