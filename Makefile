.PHONY: build test run-server run-tui clean

build:
	go build -o miniledger .

test:
	go test ./...

run-server: build
	./miniledger serve

run-tui: build
	./miniledger tui

clean:
	rm -f miniledger ledger.db
