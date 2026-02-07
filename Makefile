.PHONY: build test run-server run-tui run-web clean

build:
	go build -o miniledger .

test:
	go test ./...

run-server: build
	./miniledger serve

run-tui: build
	./miniledger tui

run-web: build
	./miniledger web

clean:
	rm -f miniledger ledger.db
