wholesale:
	GOOS=js GOARCH=wasm go build -o static/main.wasm ./cmd/wholesale

server: wholesale
	go build ./cmd/server

run: server
	./server

.PHONY: wholesale run server
