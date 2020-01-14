package config

import (
	"fmt"
	"testing"
)

func TestCheckFlagConfig(t *testing.T) {
	conf, _ := NewDefaultConfig()
	testData := ""

	fmt.Print(conf, testData)
}
