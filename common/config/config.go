package config

import (
	"flag"

	"github.com/btcsuite/btcd/chaincfg"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

const (
	DefaultConenctPeer = "192.168.1.230:8333"
)

var (
	Set *Config
)

// Config is app of conig
type Config struct {
	// Network parameters. Set mainnet, testnet, or regtest using this.
	NodeConfig NodeConfig `mapstructure:"node" json:"node"`
	P2PConfig  P2PConfig  `mapstructure:"p2p" json:"p2p"`
	RESTConfig RESTConfig `mapstructure:"rest" json:"rest"`
}

type NodeConfig struct {
	Testnet   bool `mapstructure:"testnet" json:"testnet"`
	PurneSize uint `mapstructure:"prune" json:"prune"`
}

type P2PConfig struct {
	Params      chaincfg.Params
	ConnectAddr string `mapstructure:"connect" json:"connect"`
}

type RESTConfig struct {
	ConnectAddr string `mapstructure:"connect" json:"connect"`
}

func init() {
	pflag.Bool("node.testnet", false, "Using testnet")
	pflag.Int("node.prune", 12, "Proune block size of this app")
}

func init() {
	// Bind p2p flags
	pflag.StringP("p2p.connect", "c", DefaultConenctPeer, "The address to connect p2p")
	pflag.StringP("rest.connect", "r", DefaultConenctPeer, "The address to connect rest")
}

// NewDefaultConfig is default config
func NewDefaultConfig() (*Config, error) {
	pflag.CommandLine.AddGoFlagSet(flag.CommandLine)
	pflag.Parse()
	viper.BindPFlags(pflag.CommandLine)
	var config Config
	Set = &config
	if err := viper.Unmarshal(&config); err != nil {
		return nil, err
	}
	if Set.NodeConfig.Testnet {
		Set.P2PConfig.Params = chaincfg.TestNet3Params
	} else {
		Set.P2PConfig.Params = chaincfg.MainNetParams
	}
	return Set, nil
}
