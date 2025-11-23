/*
Copyright © 2021 Miga Labs
*/
package crawler

import (
	"context"
	"crypto/ecdsa"
	"time"

	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/prometheus/client_golang/prometheus"
	cli "github.com/urfave/cli/v2"

	"github.com/migalabs/armiarma/pkg/config"
	psql "github.com/migalabs/armiarma/pkg/db/postgresql"
	"github.com/migalabs/armiarma/pkg/discovery"
	"github.com/migalabs/armiarma/pkg/discovery/discv4"
	"github.com/migalabs/armiarma/pkg/hosts"
	"github.com/migalabs/armiarma/pkg/metrics"
	"github.com/migalabs/armiarma/pkg/networks/polygon"
	"github.com/migalabs/armiarma/pkg/peering"
	"github.com/migalabs/armiarma/pkg/utils"
	"github.com/migalabs/armiarma/pkg/utils/apis"
	log "github.com/sirupsen/logrus"
)

// PolygonCrawler is the crawler for Polygon/Bor network
type PolygonCrawler struct {
	ctx       context.Context
	cancel    context.CancelFunc
	Host      *hosts.BasicLibp2pHost
	DB        *psql.DBClient
	Disc      *discovery.Discovery
	Peering   peering.PeeringService
	IpLocator *apis.IpLocator
	Metrics   *metrics.PrometheusMetrics
}

func NewPolygonCrawler(mainCtx *cli.Context, conf config.PolygonCrawlerConfig) (*PolygonCrawler, error) {
	// Setup the configuration
	log.SetLevel(utils.ParseLogLevel(conf.LogLevel))

	ctx, cancel := context.WithCancel(mainCtx.Context)
	var err error

	// Parse or create a private key for the host
	var gethPrivKey *ecdsa.PrivateKey
	var libp2pPrivKey crypto.PrivKey
	if conf.PrivateKey == "" {
		gethPrivKey, err = utils.GenerateECDSAPrivKey()
		if err != nil {
			cancel()
			return nil, err
		}
	} else {
		gethPrivKey, err = utils.ParseECDSAPrivateKey(conf.PrivateKey)
		if err != nil {
			cancel()
			return nil, err
		}
	}
	libp2pPrivKey, err = utils.AdaptSecp256k1FromECDSA(gethPrivKey)
	if err != nil {
		cancel()
		return nil, err
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

	// Create a local Polygon node
	polygonNode := polygon.NewLocalPolygonNode()

	// Generate libp2p host
	host, err := hosts.NewBasicLibp2pEth2Host(
		ctx,
		conf.IP,
		conf.Port,
		libp2pPrivKey,
		conf.UserAgent,
		polygonNode,
		ipLocator,
	)
	if err != nil {
		cancel()
		return nil, err
	}

	// Create a new discovery4 service to discover peers in the Polygon network
	dv4, err := discv4.NewDiscovery4(
		ctx,
		gethPrivKey,
		discv4.ParseBootnodesFromStringSlice(conf.Bootnodes),
		conf.Port,
		conf.Network(),
	)
	if err != nil {
		cancel()
		return nil, err
	}
	disc := discovery.NewDiscovery(
		ctx,
		dv4,
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

	// Generate the PolygonCrawler
	crawler := &PolygonCrawler{
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

	discoveryMetricsMod := disc.GetPolygonMetrics()
	promethMetrics.AddMeticsModule(discoveryMetricsMod)

	hostMetricsMod := host.GetMetrics()
	promethMetrics.AddMeticsModule(hostMetricsMod)

	return crawler, nil
}

// Run starts all the crawler subroutines
func (c *PolygonCrawler) Run() {
	// Initialization sequence for the crawler
	c.IpLocator.Run()
	c.Host.Start()
	c.Disc.Start()
	c.Peering.Run()
	c.Metrics.Start()
}

// Close shuts down the crawler gracefully
func (c *PolygonCrawler) Close() {
	c.Disc.Stop()
	c.Host.Host().Close()
	c.DB.Close()
	c.Metrics.Close()
	c.cancel()
}

// GetMetrics returns the metrics module for the crawler
func (c *PolygonCrawler) GetMetrics() *metrics.MetricsModule {
	metricsMod := metrics.NewMetricsModule(
		"crawler",
		"Polygon Crawler metrics",
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

func (c *PolygonCrawler) clientDistributionMetrics() *metrics.IndvMetrics {
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

func (c *PolygonCrawler) versionDistributionMetrics() *metrics.IndvMetrics {
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

func (c *PolygonCrawler) geoDistributionMetrics() *metrics.IndvMetrics {
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

func (c *PolygonCrawler) nodeDistributionMetrics() *metrics.IndvMetrics {
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

func (c *PolygonCrawler) deprecatedNodeMetrics() *metrics.IndvMetrics {
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

func (c *PolygonCrawler) getPeersOs() *metrics.IndvMetrics {
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

func (c *PolygonCrawler) getPeersArch() *metrics.IndvMetrics {
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

func (c *PolygonCrawler) getHostedPeers() *metrics.IndvMetrics {
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

func (c *PolygonCrawler) getRTTDist() *metrics.IndvMetrics {
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

func (c *PolygonCrawler) getIPDist() *metrics.IndvMetrics {
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
