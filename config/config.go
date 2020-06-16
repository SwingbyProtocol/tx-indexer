package config

import (
	"flag"

	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

const (
	DefaultBTCNode         = "http://192.168.1.230:8332"
	DefautlBTCWatchAddress = "mr6ioeUxNMoavbr2VjaSbPAovzzgDT7Su9"
	DefaultETHNode         = "http://192.168.1.230:8332"
	DefautlETHWatchAddress = "0x3Ec6671171710F13a1a980bc424672d873b38808"
	DefaultERC20Token      = "0xaff4481d10270f50f203e0763e2597776068cbc5"
	DefaultBNCNode         = "tcp://192.168.1.146:26657"
	DefautlBNCWatchAddress = "tbnb1ws2z8n9ygrnaeqwng69cxfpnundneyjze9cjsy"
	DefaultAccessToken     = "swingbyswingby"
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
	// Logger
	pflag.String("log.level", "info", "The log level")
	// Set BTC Network
	pflag.Bool("btc.testnet", false, "This is a btc testnet")
	// The trusted btc node
	pflag.String("btc.node", DefaultBTCNode, "The address for connect btc fullnode")
	// The target number of outbound peers. Defaults to 25.
	pflag.Int("btc.nodeSize", 25, "The maximum node count for connect p2p")
	// The watch address
	pflag.String("btc.watch", DefautlBTCWatchAddress, "The address for watch address")

	// Set ETH config
	pflag.String("eth.node", DefaultETHNode, "The address for connect eth fullnode")

	pflag.Bool("eth.testnet", false, "This is a eth testnet (Goerli)")

	pflag.String("eth.token", DefaultERC20Token, "The address for watch address")

	// Set BNB config
	pflag.String("bnc.node", DefaultBNCNode, "The address for connect bnc fullnode")

	pflag.Bool("bnc.testnet", false, "This is a bnc testnet")

	pflag.String("bnc.watch", DefautlBNCWatchAddress, "This is a bnc testnet")

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
	return &config, nil
}
