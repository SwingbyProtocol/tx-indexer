package config

import (
	"fmt"
	"testing"
)

func TestCheckFlagConfig(t *testing.T) {
	conf, _ := NewDefaultConfig()
	testData := ""
	if conf.P2PConfig.ConnAddr != testData {
		t.Fatalf("Expected to be '%s' but got '%s'", testData, conf.P2PConfig.ConnAddr)
	}
	fmt.Print(conf)
}
