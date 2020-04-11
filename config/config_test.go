package config

import (
	"fmt"
	"testing"
)

func TestCheckFlagConfig(t *testing.T) {
	conf, _ := NewDefaultConfig()
	testData := "info"
	if conf.LogConfig.LogLevel != testData {
		t.Fatalf("Expected to be '%s' but got '%s'", testData, conf.LogConfig.LogLevel)
	}
	fmt.Print(conf)
}
