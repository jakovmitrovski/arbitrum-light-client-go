run-client:
	go run ./cmd/client

run-prover:
	go run ./cmd/prover

build:
	go build -o bin/client ./cmd/client
	go build -o bin/prover ./cmd/prover
