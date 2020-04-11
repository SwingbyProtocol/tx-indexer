package config

import (
	"flag"

	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

const (
	DefaultBTCNode = "http://192.168.1.230:8332"
)

// Config is app of conig
type Config struct {
	// Network parameters. Set mainnet, testnet, or regtest using this.
	BTCConfig  BTCConfig  `mapstructure:"btc" json:"node"`
	LogConfig  LogConfig  `mapstructure:"log" json:"log"`
	RESTConfig RESTConfig `mapstructure:"rest" json:"rest"`
	WSConfig   WSConfig   `mapstructure:"ws" json:"ws"`
}

type BTCConfig struct {
	NodeAddr       string `mapstructure:"node" json:"node"`
	TargetOutbound uint32 `mapstructure:"targetSize" json:"targetSize"`
}

type LogConfig struct {
	LogLevel string `mapstructure:"level" json:"level"`
}

type P2PConfig struct {
	ConnAddr       string `mapstructure:"connect" json:"connect"`
	TargetOutbound uint32 `mapstructure:"targetSize" json:"targetSize"`
}

type RESTConfig struct {
	ConnAddr   string `mapstructure:"connect" json:"connect"`
	ListenAddr string `mapstructure:"listen" json:"listen"`
}

type WSConfig struct {
	ListenAddr string `mapstructure:"listen" json:"listen"`
}

func init() {
	pflag.String("log.level", "info", "The log level")
	// Bind btc configs
	pflag.StringP("btc.node", "c", DefaultBTCNode, "The address for connect block finalizer")
	pflag.StringP("rest.listen", "l", "0.0.0.0:9096", "The listen address for REST API")
	// Bind p2p flags
	pflag.String("p2p.connect", "", "The address for connect p2p network")
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
	return &config, nil
}
