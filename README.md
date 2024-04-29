# wholesale

Roblox deployment downloader written in Go using [rbxbin](https://github.com/apprehensions/rbxbin), with a CLI and a WASM frontend.

### Usage (CLI)
```
go install github.com/vinegarhq/wholesale@latest
PATH="$PATH:$(go env GOPATH)"
wholesale -guid version-24872f7beace4d0a
```

### Usage (Web)
Navigate to the WASM implementation at [wholesale.vinegarhq.org](https://wholesale.vinegarhq.org/), an example usage will be given.
