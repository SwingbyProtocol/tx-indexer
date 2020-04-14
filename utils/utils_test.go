package utils

import (
	"encoding/hex"
	"testing"

	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/txscript"
)

func TestScriptToPubkeyInfo(t *testing.T) {

	mainetParams := chaincfg.MainNetParams

	testPubKeyHash := "76a9141a2553392ba26892c4d5eba55cc23ecbd81350ad88ac"

	testAddress := "13PFHP4crS6xjR6mm8xaDNPRSnzjjs9msP"

	testReqSig := int(1)

	testScriptClass := txscript.PubKeyHashTy.String()

	script, _ := hex.DecodeString(testPubKeyHash)

	spi, _ := ScriptToPubkeyInfo(script, &mainetParams)

	// TODO: support ASM
	if spi.Asm != "" {
		t.Fatalf("Expected it to be '%s' but got '%s'", testPubKeyHash, spi.Hex)
	}

	if spi.Hex != testPubKeyHash {
		t.Fatalf("Expected it to be '%s' but got '%s'", testPubKeyHash, spi.Hex)
	}

	if spi.Reqsigs != testReqSig {
		t.Fatalf("Expected it to be '%d' but got '%d'", testReqSig, spi.Reqsigs)
	}

	if spi.ScriptClass != testScriptClass {
		t.Fatalf("Expected it to be '%s' but got '%s'", testScriptClass, spi.ScriptClass)
	}

	if spi.Addresses[0] != testAddress {
		t.Fatalf("Expected it to be '%s' but got '%s'", testAddress, spi.Addresses[0])
	}
}
