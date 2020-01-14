package config

import (
	"flag"

	"github.com/btcsuite/btcd/chaincfg"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

// Config is app of conig
type Config struct {
	// Network parameters. Set mainnet, testnet, or regtest using this.
	NodeConfig NodeConfig `mapstructure:"node" json:"node"`
	P2PConfig  P2PConfig  `mapstructure:"p2p" json:"p2p"`
	RESTConfig RESTConfig `mapstructure:"rest" json:"rest"`
	WSConfig   WSConfig   `mapstructure:"ws" json:"ws"`
}

type NodeConfig struct {
	Testnet   bool   `mapstructure:"testnet" json:"testnet"`
	LogLevel  string `mapstructure:"loglevel" json:"loglevel"`
	PurneSize uint   `mapstructure:"prune" json:"prune"`
}

type P2PConfig struct {
	Params         *chaincfg.Params
	TargetOutbound uint32 `mapstructure:"targetSize" json:"targetSize"`
}

type RESTConfig struct {
	Finalizer  string `mapstructure:"finalizer" json:"finalizer"`
	ListenAddr string `mapstructure:"listen" json:"listen"`
}

type WSConfig struct {
	ListenAddr string `mapstructure:"listen" json:"listen"`
}

func init() {
	pflag.Bool("node.testnet", false, "Using testnet")
	pflag.String("node.loglevel", "info", "The loglevel")
	// Bind rest flags
	pflag.StringP("rest.finalizer", "c", "http://192.168.1.230:8332", "The address for finalizer")
	pflag.StringP("rest.listen", "l", "0.0.0.0:9096", "The listen address for REST API")
	// Bind p2p flags
	pflag.Int("p2p.targetSize", 25, "The maximum node count for connect p2p")
	// Bind ws flags
	pflag.StringP("ws.listen", "w", "0.0.0.0:9099", "The listen address for Websocket API")
}

// NewDefaultConfig is default config
func NewDefaultConfig() (*Config, error) {
	pflag.CommandLine.AddGoFlagSet(flag.CommandLine)
	pflag.Parse()
	viper.BindPFlags(pflag.CommandLine)
	var config Config
	if err := viper.Unmarshal(&config); err != nil {
		return nil, err
	}
	if config.NodeConfig.Testnet {
		config.P2PConfig.Params = &chaincfg.TestNet3Params
	} else {
		config.P2PConfig.Params = &chaincfg.MainNetParams
	}
	return &config, nil
}
