/*
Copyright © 2021 Miga Labs
*/
package crawler

import (
	"context"
	"crypto/ecdsa"
	"time"

	"github.com/libp2p/go-libp2p/core/crypto"
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
	return metrics.NewMetricsModule(
		"polygon_crawler",
		"Polygon Crawler metrics",
	)
}
