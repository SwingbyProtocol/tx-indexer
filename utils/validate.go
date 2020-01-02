package utils

import (
	"errors"
	"fmt"

	"github.com/btcsuite/btcd/blockchain"
	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcd/wire"
	"github.com/btcsuite/btcutil"
)

const (
	maxStandardMultiSigKeys  = 3
	maxTxVersion             = int32(2)
	maxStandardTxWeight      = int64(400000)
	maxStandardSigScriptSize = 1650
	minRelayTxFee            = 0
)

func OutputRejector(msgTx *wire.MsgTx) error {
	var isError error
	for _, vout := range msgTx.TxOut {
		if vout.Value < 0 {
			isError = errors.New("vout amount is negative")
		}
	}
	return isError
}

func CheckNonStandardTx(tx *btcutil.Tx) error {
	msgTx := tx.MsgTx()
	if msgTx.Version > maxTxVersion || msgTx.Version < 1 {
		str := fmt.Sprintf("transaction version %d is not in the "+
			"valid range of %d-%d", msgTx.Version, 1,
			maxTxVersion)
		return errors.New(str)
	}

	// The transaction must be finalized to be standard and therefore
	// considered for inclusion in a block.
	//if !blockchain.IsFinalizedTransaction(tx, height, medianTimePast) {
	//	return txRuleError(wire.RejectNonstandard,
	//		"transaction is not finalized")
	//}

	// Since extremely large transactions with a lot of inputs can cost
	// almost as much to process as the sender fees, limit the maximum
	// size of a transaction.  This also helps mitigate CPU exhaustion
	// attacks.
	txWeight := GetTransactionWeight(msgTx)
	if txWeight > maxStandardTxWeight {
		str := fmt.Sprintf("weight of transaction %v is larger than max "+
			"allowed weight of %v", txWeight, maxStandardTxWeight)
		return errors.New(str)
	}

	for i, txIn := range msgTx.TxIn {
		// Each transaction input signature script must not exceed the
		// maximum size allowed for a standard transaction.  See
		// the comment on maxStandardSigScriptSize for more details.
		sigScriptLen := len(txIn.SignatureScript)
		if sigScriptLen > maxStandardSigScriptSize {
			str := fmt.Sprintf("transaction input %d: signature "+
				"script size of %d bytes is large than max "+
				"allowed size of %d bytes", i, sigScriptLen,
				maxStandardSigScriptSize)
			return errors.New(str)
		}

		// Each transaction input signature script must only contain
		// opcodes which push data onto the stack.
		if !txscript.IsPushOnlyScript(txIn.SignatureScript) {
			str := fmt.Sprintf("transaction input %d: signature "+
				"script is not push only", i)
			return errors.New(str)
		}
	}

	// None of the output public key scripts can be a non-standard script or
	// be "dust" (except when the script is a null data script).
	numNullDataOutputs := 0
	for i, txOut := range msgTx.TxOut {
		scriptClass := txscript.GetScriptClass(txOut.PkScript)
		err := checkPkScriptStandard(txOut.PkScript, scriptClass)
		if err != nil {
			// Attempt to extract a reject code from the error so
			// it can be retained.  When not possible, fall back to
			// a non standard error.
			//rejectCode := wire.RejectNonstandard
			str := fmt.Sprintf("transaction output %d: %v", i, err)
			return errors.New(str)
		}

		// Accumulate the number of outputs which only carry data.  For
		// all other script types, ensure the output value is not
		// "dust".
		if scriptClass == txscript.NullDataTy {
			numNullDataOutputs++
		} else if isDust(txOut, minRelayTxFee) {
			str := fmt.Sprintf("transaction output %d: payment "+
				"of %d is dust", i, txOut.Value)
			return errors.New(str)
		}
	}

	// A standard transaction must not have more than one output script that
	// only carries data.
	if numNullDataOutputs > 1 {
		str := "more than one transaction output in a nulldata script"
		return errors.New(str)
	}

	return nil
}

func isDust(txOut *wire.TxOut, minRelayTxFee btcutil.Amount) bool {
	// Unspendable outputs are considered dust.
	if txscript.IsUnspendable(txOut.PkScript) {
		return true
	}

	// The total serialized size consists of the output and the associated
	// input script to redeem it.  Since there is no input script
	// to redeem it yet, use the minimum size of a typical input script.
	//
	// Pay-to-pubkey-hash bytes breakdown:
	//
	//  Output to hash (34 bytes):
	//   8 value, 1 script len, 25 script [1 OP_DUP, 1 OP_HASH_160,
	//   1 OP_DATA_20, 20 hash, 1 OP_EQUALVERIFY, 1 OP_CHECKSIG]
	//
	//  Input with compressed pubkey (148 bytes):
	//   36 prev outpoint, 1 script len, 107 script [1 OP_DATA_72, 72 sig,
	//   1 OP_DATA_33, 33 compressed pubkey], 4 sequence
	//
	//  Input with uncompressed pubkey (180 bytes):
	//   36 prev outpoint, 1 script len, 139 script [1 OP_DATA_72, 72 sig,
	//   1 OP_DATA_65, 65 compressed pubkey], 4 sequence
	//
	// Pay-to-pubkey bytes breakdown:
	//
	//  Output to compressed pubkey (44 bytes):
	//   8 value, 1 script len, 35 script [1 OP_DATA_33,
	//   33 compressed pubkey, 1 OP_CHECKSIG]
	//
	//  Output to uncompressed pubkey (76 bytes):
	//   8 value, 1 script len, 67 script [1 OP_DATA_65, 65 pubkey,
	//   1 OP_CHECKSIG]
	//
	//  Input (114 bytes):
	//   36 prev outpoint, 1 script len, 73 script [1 OP_DATA_72,
	//   72 sig], 4 sequence
	//
	// Pay-to-witness-pubkey-hash bytes breakdown:
	//
	//  Output to witness key hash (31 bytes);
	//   8 value, 1 script len, 22 script [1 OP_0, 1 OP_DATA_20,
	//   20 bytes hash160]
	//
	//  Input (67 bytes as the 107 witness stack is discounted):
	//   36 prev outpoint, 1 script len, 0 script (not sigScript), 107
	//   witness stack bytes [1 element length, 33 compressed pubkey,
	//   element length 72 sig], 4 sequence
	//
	//
	// Theoretically this could examine the script type of the output script
	// and use a different size for the typical input script size for
	// pay-to-pubkey vs pay-to-pubkey-hash inputs per the above breakdowns,
	// but the only combination which is less than the value chosen is
	// a pay-to-pubkey script with a compressed pubkey, which is not very
	// common.
	//
	// The most common scripts are pay-to-pubkey-hash, and as per the above
	// breakdown, the minimum size of a p2pkh input script is 148 bytes.  So
	// that figure is used. If the output being spent is a witness program,
	// then we apply the witness discount to the size of the signature.
	//
	// The segwit analogue to p2pkh is a p2wkh output. This is the smallest
	// output possible using the new segwit features. The 107 bytes of
	// witness data is discounted by a factor of 4, leading to a computed
	// value of 67 bytes of witness data.
	//
	// Both cases share a 41 byte preamble required to reference the input
	// being spent and the sequence number of the input.
	totalSize := txOut.SerializeSize() + 41
	if txscript.IsWitnessProgram(txOut.PkScript) {
		totalSize += (107 / blockchain.WitnessScaleFactor)
	} else {
		totalSize += 107
	}

	// The output is considered dust if the cost to the network to spend the
	// coins is more than 1/3 of the minimum free transaction relay fee.
	// minFreeTxRelayFee is in Satoshi/KB, so multiply by 1000 to
	// convert to bytes.
	//
	// Using the typical values for a pay-to-pubkey-hash transaction from
	// the breakdown above and the default minimum free transaction relay
	// fee of 1000, this equates to values less than 546 satoshi being
	// considered dust.
	//
	// The following is equivalent to (value/totalSize) * (1/3) * 1000
	// without needing to do floating point math.
	return txOut.Value*1000/(3*int64(totalSize)) < int64(minRelayTxFee)
}

func checkPkScriptStandard(pkScript []byte, scriptClass txscript.ScriptClass) error {
	switch scriptClass {
	case txscript.MultiSigTy:
		numPubKeys, numSigs, err := txscript.CalcMultiSigStats(pkScript)
		if err != nil {
			str := fmt.Sprintf("multi-signature script parse "+
				"failure: %v", err)
			return errors.New(str)
		}

		// A standard multi-signature public key script must contain
		// from 1 to maxStandardMultiSigKeys public keys.
		if numPubKeys < 1 {
			str := "multi-signature script with no pubkeys"
			return errors.New(str)
		}
		if numPubKeys > maxStandardMultiSigKeys {
			str := fmt.Sprintf("multi-signature script with %d "+
				"public keys which is more than the allowed "+
				"max of %d", numPubKeys, maxStandardMultiSigKeys)
			return errors.New(str)
		}

		// A standard multi-signature public key script must have at
		// least 1 signature and no more signatures than available
		// public keys.
		if numSigs < 1 {
			return errors.New("multi-signature script with no signatures")
		}
		if numSigs > numPubKeys {
			str := fmt.Sprintf("multi-signature script with %d "+
				"signatures which is more than the available "+
				"%d public keys", numSigs, numPubKeys)
			return errors.New(str)
		}

	case txscript.NonStandardTy:
		return errors.New("non-standard script form")
	}

	return nil
}
