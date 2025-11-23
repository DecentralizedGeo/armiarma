package config

import (
	"github.com/migalabs/armiarma/pkg/utils"
	log "github.com/sirupsen/logrus"
	cli "github.com/urfave/cli/v2"
)

var (
	// Default Celo-specific settings
	DefaultCeloGossipTopics []string = []string{}
)

type CeloCrawlerConfig struct {
	LogLevel                  string   `json:"log-level"`
	PrivateKey                string   `json:"priv-key"`
	IP                        string   `json:"ip"`
	Port                      int      `json:"port"`
	MetricsIP                 string   `json:"metrics-ip"`
	MetricsPort               int      `json:"metrics-port"`
	UserAgent                 string   `json:"user-agent"`
	PsqlEndpoint              string   `json:"psql-endpoint"`
	ActivePeersBackupInterval string   `json:"active-peers-backup-interval"`
	Bootnodes                 []string `json:"bootnodes"`
	PersistConnEvents         bool     `json:"persist-connevents"`
}

// NewCeloCrawlerConfig returns a new CeloCrawlerConfig with default values
func NewCeloCrawlerConfig() *CeloCrawlerConfig {
	return &CeloCrawlerConfig{
		LogLevel:                  DefaultLogLevel,
		PrivateKey:                DefaultPrivKey,
		IP:                        DefaultIP,
		Port:                      DefaultPort,
		MetricsIP:                 DefaultMetricsIP,
		MetricsPort:               DefaultMetricsPort,
		UserAgent:                 DefaultUserAgent,
		PsqlEndpoint:              DefaultPSQLEndpoint,
		ActivePeersBackupInterval: DefaultActivePeersBackupInterval,
		Bootnodes:                 DefaultCeloBootnodes,
		PersistConnEvents:         DefaultPersistConnEvents,
	}
}

func (c *CeloCrawlerConfig) Apply(ctx *cli.Context) {
	// Apply CLI flags to the configuration
	
	// log level
	if ctx.IsSet("log-level") {
		c.LogLevel = ctx.String("log-level")
	}
	
	// private key
	if ctx.IsSet("priv-key") {
		c.PrivateKey = ctx.String("priv-key")
	}
	
	// ip
	if ctx.IsSet("ip") {
		c.IP = ctx.String("ip")
	}
	
	// port
	if ctx.IsSet("port") {
		port := ctx.Int("port")
		if checkValidPort(port) {
			c.Port = port
		}
	}
	
	// metrics-ip
	if ctx.IsSet("metrics-ip") {
		c.MetricsIP = ctx.String("metrics-ip")
	}
	
	// metrics-port
	if ctx.IsSet("metrics-port") {
		mPort := ctx.Int("metrics-port")
		if checkValidPort(mPort) {
			c.MetricsPort = mPort
		}
	}
	
	// user agent
	if ctx.IsSet("user-agent") {
		c.UserAgent = ctx.String("user-agent")
	}
	
	// postgresql endpoint
	if ctx.IsSet("psql-endpoint") {
		c.PsqlEndpoint = ctx.String("psql-endpoint")
	}
	
	// active peers' backup interval
	if ctx.IsSet("peers-backup") {
		c.ActivePeersBackupInterval = ctx.String("peers-backup")
	}
	
	// bootnodes
	if ctx.IsSet("bootnode") {
		c.Bootnodes = ctx.StringSlice("bootnode")
	}
	
	// persist connection events
	if ctx.IsSet("persist-connevents") {
		c.PersistConnEvents = ctx.Bool("persist-connevents")
	}

	log.WithFields(log.Fields{
		"log-level":          c.LogLevel,
		"priv-key":           c.PrivateKey,
		"ip":                 c.IP,
		"port":               c.Port,
		"user-agent":         c.UserAgent,
		"psql":               c.PsqlEndpoint,
		"backup-interval":    c.ActivePeersBackupInterval,
		"bootnodes":          c.Bootnodes,
		"persist-connevents": c.PersistConnEvents,
	}).Info("config for the Celo crawler")
}

func (c *CeloCrawlerConfig) Network() utils.NetworkType {
	return utils.CeloNetwork
}

