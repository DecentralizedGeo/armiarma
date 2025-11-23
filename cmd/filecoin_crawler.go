/*
Copyright © 2021 Miga Labs
*/
package cmd

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	log "github.com/sirupsen/logrus"
	cli "github.com/urfave/cli/v2"

	"github.com/migalabs/armiarma/pkg/config"
	"github.com/migalabs/armiarma/pkg/crawler"
)

// FilecoinCrawlerCommand contains the filecoin sub-command configuration
var FilecoinCrawlerCommand = &cli.Command{
	Name:   "filecoin",
	Usage:  "crawl the Filecoin network",
	Action: LaunchFilecoinCrawler,
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:        "log-level",
			Usage:       "Verbosity level for the Crawler's logs",
			EnvVars:     []string{"ARMIARMA_LOG_LEVEL"},
			DefaultText: config.DefaultLogLevel,
		},
		&cli.StringFlag{
			Name:    "priv-key",
			Usage:   "String representation of the PrivateKey to be used by the crawler",
			EnvVars: []string{"ARMIARMA_PRIV_KEY"},
		},
		&cli.StringFlag{
			Name:        "ip",
			Usage:       "IP in the machine that we want to assign to the crawler",
			EnvVars:     []string{"ARMIARMA_IP"},
			DefaultText: config.DefaultIP,
		},
		&cli.IntFlag{
			Name:        "port",
			Usage:       "TCP and UDP port that the crawler will advertise to establish connections",
			EnvVars:     []string{"ARMIARMA_PORT"},
			DefaultText: fmt.Sprintf("%d", config.DefaultPort),
		},
		&cli.StringFlag{
			Name:        "metrics-ip",
			Usage:       "IP in the machine that will expose the metrics of the crawler",
			EnvVars:     []string{"ARMIARMA_METRICS_IP"},
			DefaultText: config.DefaultMetricsIP,
		},
		&cli.IntFlag{
			Name:        "metrics-port",
			Usage:       "Port that the crawler will use to expose pprof and prometheus metrics",
			EnvVars:     []string{"ARMIARMA_METRICS_PORT"},
			DefaultText: fmt.Sprintf("%d", config.DefaultMetricsPort),
		},
		&cli.StringFlag{
			Name:        "user-agent",
			Usage:       "Agent name that will identify the crawler in the network",
			EnvVars:     []string{"ARMIARMA_USER_AGENT"},
			DefaultText: config.DefaultUserAgent,
		},
		&cli.StringFlag{
			Name:        "psql-endpoint",
			Usage:       "PSQL endpoint where the crawler will submit all the gathered info",
			EnvVars:     []string{"ARMIARMA_PSQL"},
			DefaultText: config.DefaultPSQLEndpoint,
		},
		&cli.StringFlag{
			Name:        "peers-backup",
			Usage:       "Time interval that will be used to backup the peer_ids into a single table",
			EnvVars:     []string{"ARMIARMA_BACKUP_INTERVAL"},
			DefaultText: config.DefaultActivePeersBackupInterval,
		},
		&cli.StringSliceFlag{
			Name:    "bootnode",
			Usage:   "List of bootnodes that the crawler will use to discover more peers in the network (One --bootnode <bootnode> per bootnode)",
			EnvVars: []string{"ARMIARMA_BOOTNODES"},
		},
		&cli.BoolFlag{
			Name:    "persist-connevents",
			Usage:   "Decide whether we want to track the connection-events into the DB (Disk intense)",
			EnvVars: []string{"ARMIARMA_PERSIST_CONNEVENTS"},
		},
	},
}

// LaunchFilecoinCrawler is the function that is called when running `filecoin`
func LaunchFilecoinCrawler(c *cli.Context) error {
	log.Infoln("Starting Filecoin Crawler...")

	conf := config.NewFilecoinCrawlerConfig()
	conf.Apply(c)

	// Generate the Filecoin crawler struct
	filecoinCrawler, err := crawler.NewFilecoinCrawler(c, *conf)
	if err != nil {
		return err
	}

	// Launch the subroutines
	filecoinCrawler.Run()

	// Check the shutdown signal
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM, os.Interrupt, syscall.SIGTERM)

	// Keep the app running until syscall.SIGTERM
	sig := <-sigs
	log.Printf("Received %s signal - Stopping...\n", sig.String())
	signal.Stop(sigs)
	filecoinCrawler.Close()

	return nil
}
