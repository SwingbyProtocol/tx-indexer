package eth

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/SwingbyProtocol/tx-indexer/chains/eth/types"
	"github.com/ethereum/go-ethereum/common"
	log "github.com/sirupsen/logrus"
)

func TestClient(t *testing.T) {

	testCase := `{
			"jsonrpc": "2.0",
			"id": 1,
			"result":{
			"pending":{
			"0x6CEc2546bdf3A970e5326D382eF01b8fe04873Dc":{
			"63080":{
			"blockHash": null,
			"blockNumber": null,
			"from": "0x6cec2546bdf3a970e5326d382ef01b8fe04873dc",
			"gas": "0x493e0",
			"gasPrice": "0xb2d05e00",
			"hash": "0x8e56d02a87456064289dcc1262838f9a082c4c3fb0e4a07a1e58440b939880a6",
			"input": "0xf80109600000000000000000000000000000000000000000000000000000000000000100000000000000000000000000000000000000000000000000000000000000018000000000000000000000000000000000000000000000000000000000000001c0000000000000000000000000000000000000000000000000000000000000026000000000000000000000000000000000000000000000000000000000000002c000000000000000000000000000000000000000000000000000000000000003200000000000000000000000000000000000000000000000000000000007270e00000000000000000000000000000000000000000000000000000000000000038000000000000000000000000000000000000000000000000000000000000000030000000000000000000000008a4575f1ca18e388b4284c98635f3f208c366287000000000000000000000000905f2ac5da6327920754ead667383989d87e94ce000000000000000000000000861fed1858194d041f38faf8a32a0a6c46e62a930000000000000000000000000000000000000000000000000000000000000001000000000000000000000000cffab51b22278939fedc8d1eca848b09f52f28ee000000000000000000000000000000000000000000000000000000000000000400000000000000000000000000000000000000000000000000000000000000010000000000000000000000000000000000000000000000000000000000000001000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000002000000000000000000000000000000000000000000000000000000000000001c000000000000000000000000000000000000000000000000000000000000001b000000000000000000000000000000000000000000000000000000000000000273d083b39205ff4f037a69e450fb414b7711dd6e12942f4b4d5d889f53cb31764e889ed54a4082d79169645ef825874e1db7739ab8c6164932525100615c261900000000000000000000000000000000000000000000000000000000000000025f343e40311413a908656f837f0186b67394172830f47a5298be0dcc329281d65c09e0284b40873ea0bc2e383b57f332070106d10cd3ba556d15799d6ceeeb990000000000000000000000000000000000000000000000000000000000000000",
			"nonce": "0xf668",
			"to": "0x0a8156e7ee392d885d10eaa86afd0e323afdcd95",
			"transactionIndex": null,
			"value": "0x0",
			"v": "0x2e",
			"r": "0x9c369e103e5b353b1788854fb650a88dcb3fe6420e29e4c3cb85495039055b37",
			"s": "0x7decbf6d924004d645505796b83d6ad5401093fe0adc32406d34a1b291484c08"
			}
			}
			},
			"queued":{}
			}
			}`

	uri := os.Getenv("ethRPC")
	var res types.MempoolResponse
	if err := json.Unmarshal([]byte(testCase), &res); err != nil {
		log.Info(err)
	}

	cli := NewClinet(uri)
	inTxs, outTxs := cli.GetMempoolTxs(common.HexToAddress("0xaff4481d10270f50f203e0763e2597776068cbc5"), common.HexToAddress("0xaff4481d10270f50f203e0763e2597776068cbc5"))
	log.Info(inTxs, outTxs)

	select {}
}
