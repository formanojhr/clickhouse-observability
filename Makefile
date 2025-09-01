PROTO=proto/logs.proto

generate:
	protoc -I proto --go_out=. --go-grpc_out=. $(PROTO)

run:
	go run ./cmd/server

test:
	go test ./...

dev:
	go mod tidy
