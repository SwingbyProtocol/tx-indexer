package node

/*
func TestConfig(t *testing.T) {

	config := &PeerManagerConfig{}
	config.Params = &chaincfg.MainNetParams

	pm, err := NewPeerManager(config)
	if err != nil {
		fmt.Print(err)
	}
	fmt.Print(pm)
	pm.Start()

	mms := <-pm.msgChan
	fmt.Print(mms)
	select {}
}


func TestPeerConfig(t *testing.T) {
	fmt.Print("eee")

	db, _ := db.Create("./")

	seed := b39.NewSeed("body jar cool knock yard era immense old alert problem scrub domain napkin flavor pen diamond quote attitude sight danger comfort repeat penalty danger", "")

	chainconfig := &chaincfg.MainNetParams

	chainconfig.ReduceMinDifficulty = true

	mPrivKey, err := hd.NewMaster(seed, chainconfig)
	if err != nil {
		fmt.Print(err)
	}

	keyManager, err := NewKeyManager(db.Keys(), chainconfig, mPrivKey)
	if err != nil {
		fmt.Print(err)
	}

	txstore, err := NewTxStore(chainconfig, db, keyManager)
	if err != nil {
		fmt.Print(err)
	}
	now := time.Now()

		blockchain, err := NewBlockchain("./", now, chainconfig)
		if err != nil {
			fmt.Print(err)
		}

	minSync := 1

	wireConfig := &WireServiceConfig{
		txStore:            txstore,
		walletCreationDate: now,
		minPeersForSync:    minSync,
		params:             txstore.params,
	}

	ws := NewWireService(wireConfig)

	config := &PeerManagerConfig{
		UserAgentName:    "spvwallet",
		UserAgentVersion: "0.1.0",
		Params:           txstore.params,
		AddressCacheDir:  "./",
		MsgChan:          ws.MsgChan(),
	}

	pm, err := NewPeerManager(config)
	if err != nil {
		fmt.Print(err)
	}

	pm.Start()
	ws.Start()

	select {}
}
*/
