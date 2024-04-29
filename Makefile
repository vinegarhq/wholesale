WHOLESALE_WASM = static/wholesale.wasm

wholesale:
	go build -o $@

$(WHOLESALE_WASM):
	GOOS=js GOARCH=wasm go build -o $@

server: $(WHOLESALE_WASM)
	go build ./cmd/server

run: server
	./server

.PHONY: wholesale run server $(WHOLESALE_WASM)
