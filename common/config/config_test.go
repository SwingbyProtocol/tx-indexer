package config

import (
	"fmt"
	"testing"
)

func TestCheckFlagConfig(t *testing.T) {
	conf, _ := NewDefaultConfig()
	testData := "0.0.0.0"
	if conf.P2PConfig.ConnAddr != testData {
		t.Fatalf("Expected config.ListenAddr to be '%s' but got '%s'", testData, conf.P2PConfig.ConnAddr)
	}
	fmt.Print(conf)
}
