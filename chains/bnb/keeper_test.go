package bnb

import (
	"testing"
)

func TestClient(t *testing.T) {

	//uri := os.Getenv("bnbRPC")

	bnbKeeper := NewKeeper("tcp://data-seed-pre-0-s3.binance.org:80", true)

	bnbKeeper.SetWatchAddr("tbnb1gls26vjjqqjw7wfgs07707yr58lc8z8sxml4hk")

	bnbKeeper.Start()
}
