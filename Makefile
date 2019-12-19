install:
		cd cmd/tx-indexer && go install

build-linux-amd64:
		cd cmd/tx-indexer && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o ../../bin/tx-indexer-linux-amd64 .


