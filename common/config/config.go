package config

import (
	"flag"
	"fmt"
	"os"
	"path"
	"runtime"
	"strings"

	"github.com/btcsuite/btcd/chaincfg"
	"github.com/sirupsen/logrus"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

const (
	DefaultConenctPeer = "http://192.168.1.230:8332"
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
	WSConfig   WSConfig   `mapstructure:"ws" json:"ws"`
}

type NodeConfig struct {
	Testnet   bool   `mapstructure:"testnet" json:"testnet"`
	LogLevel  string `mapstructure:"loglevel" json:"loglevel"`
	PurneSize uint   `mapstructure:"prune" json:"prune"`
}

type P2PConfig struct {
	Params         chaincfg.Params
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
	log.SetOutput(os.Stdout)
	log.SetReportCaller(true)
	log.SetFormatter(&logrus.TextFormatter{
		FullTimestamp: true,
		CallerPrettyfier: func(f *runtime.Frame) (string, string) {
			s := strings.Split(f.Function, ".")
			funcname := s[len(s)-1]
			_, filename := path.Split(f.File)
			paddedFuncname := fmt.Sprintf("%-20v", funcname+"()")
			paddedFilename := fmt.Sprintf("%17v", filename)
			return paddedFuncname, paddedFilename
		},
	})
}

func init() {
	pflag.Bool("node.testnet", false, "Using testnet")
	pflag.Int("node.prune", 12, "Proune block size of this app")
	pflag.String("node.loglevel", "info", "The loglevel")
}

func init() {
	// Bind rest flags
	pflag.StringP("rest.connect", "c", DefaultConenctPeer, "The address to connect rest")
	pflag.StringP("rest.listen", "l", "0.0.0.0:9096", "The listen address for REST API")
	// Bind p2p flags
	pflag.String("p2p.connect", "", "The address to connect p2p")
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
	Set = &config
	if err := viper.Unmarshal(&config); err != nil {
		return nil, err
	}
	if Set.NodeConfig.Testnet {
		Set.P2PConfig.Params = chaincfg.TestNet3Params
	} else {
		Set.P2PConfig.Params = chaincfg.MainNetParams
	}

	loglevel := config.NodeConfig.LogLevel
	if loglevel == "debug" {
		log.SetLevel(log.DebugLevel)
	}
	return Set, nil
}
