WHOLESALE_WASM = static/wholesale.wasm

all: wholesale

wasm: $(WHOLESALE_WASM)

wholesale:
	go build -o $@

$(WHOLESALE_WASM):
	GOOS=js GOARCH=wasm go build -o $@

clean:
	rm -f wholesale $(WHOLESALE_WASM)

.PHONY: wholesale wasm clean
