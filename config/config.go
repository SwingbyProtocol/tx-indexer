package config

import (
	"errors"
	"flag"

	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

const (
	DefaultBTCNode    = "http://bitcoinrpc:DVnnAgN2vYvgS26UR8lH9QJVgbq5nFerycMuwh8P7Sw@192.168.1.146:18332"
	DefaultETHNode    = "http://192.168.1.146:8545"
	DefaultERC20Token = "0xaff4481d10270f50f203e0763e2597776068cbc5"
	DefaultBNCNode    = "tcp://192.168.1.146:26657"
)

// Config is app of conig
type Config struct {
	// Network parameters. Set mainnet, testnet, or regtest using this.
	BTCConfig  BTCConfig  `mapstructure:"btc" json:"btc"`
	ETHConfig  ETHConfig  `mapstructure:"eth" json:"eth"`
	BNCConfig  BNCConfig  `mapstructure:"bnc" json:"bnc"`
	LogConfig  LogConfig  `mapstructure:"log" json:"log"`
	RESTConfig RESTConfig `mapstructure:"rest" json:"rest"`
	WSConfig   WSConfig   `mapstructure:"ws" json:"ws"`
	PruneTime  int64      `mapstructure:"prune" json:"prune"`
}

type BTCConfig struct {
	NodeAddr       string `mapstructure:"node" json:"node"`
	Testnet        bool   `mapstructure:"testnet" json:"testnet"`
	TargetOutbound uint32 `mapstructure:"nodeSize" json:"targetSize"`
}

type ETHConfig struct {
	NodeAddr   string `mapstructure:"node" json:"node"`
	WatchToken string `mapstructure:"token" json:"token"`
	Testnet    bool   `mapstructure:"testnet" json:"testnet"`
}

type BNCConfig struct {
	NodeAddr string `mapstructure:"node" json:"node"`
	Testnet  bool   `mapstructure:"testnet" json:"testnet"`
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
	// Prune
	pflag.Int("prune", 24, "Prune time (hours)")
	// Logger
	pflag.String("log.level", "info", "The log level")
	// Set BTC Network
	pflag.Bool("btc.testnet", false, "This is a flag to set testnet-mode on btc network")
	// The trusted btc node
	pflag.String("btc.node", DefaultBTCNode, "The address for connect btc fullnode")
	// The target number of outbound peers. Defaults to 25.
	pflag.Int("btc.nodeSize", 25, "The maximum node count for connect p2p fullnode")
	// Set ETH config
	pflag.String("eth.node", DefaultETHNode, "The address for connect eth fullnode")

	pflag.Bool("eth.testnet", false, "This is a flag to set testnet-mode on eth network")

	pflag.String("eth.token", DefaultERC20Token, "The token address for watch")
	// Set BNB config
	pflag.String("bnc.node", DefaultBNCNode, "The address for connect bnc fullnode")

	pflag.Bool("bnc.testnet", false, "This is a flag to set testnet-mode on bnc network")

	pflag.StringP("rest.listen", "l", "0.0.0.0:9096", "The listen address for REST API")
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
	if config.PruneTime < 1 {
		return nil, errors.New("prune time should be > 1 hours")
	}
	return &config, nil
}
