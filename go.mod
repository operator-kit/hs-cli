module github.com/operator-kit/hs-cli

go 1.25.0

require (
	github.com/JohannesKaufmann/html-to-markdown/v2 v2.5.0
	github.com/shota3506/onnxruntime-purego v0.0.0-20251207004809-1c85186598a5
	github.com/spf13/cobra v1.10.2
	github.com/spf13/pflag v1.0.9
	github.com/stretchr/testify v1.11.1
	github.com/sugarme/tokenizer v0.3.0
	github.com/zalando/go-keyring v0.2.6
	golang.org/x/oauth2 v0.35.0
	golang.org/x/time v0.14.0
	gopkg.in/yaml.v3 v3.0.1
)

// Patch: fix runtime.AddCleanup panic — cleanup closure captures *Value ptr,
// preventing GC from collecting it. See patches/onnxruntime-purego/onnxruntime/value.go.
replace github.com/shota3506/onnxruntime-purego => ./patches/onnxruntime-purego

require (
	al.essio.dev/pkg/shellescape v1.5.1 // indirect
	github.com/JohannesKaufmann/dom v0.2.0 // indirect
	github.com/danieljoos/wincred v1.2.2 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/ebitengine/purego v0.10.0 // indirect
	github.com/emirpasic/gods v1.18.1 // indirect
	github.com/godbus/dbus/v5 v5.1.0 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/mitchellh/colorstring v0.0.0-20190213212951-d06e56a500db // indirect
	github.com/patrickmn/go-cache v2.1.0+incompatible // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/rivo/uniseg v0.4.7 // indirect
	github.com/schollz/progressbar/v2 v2.15.0 // indirect
	github.com/sugarme/regexpset v0.0.0-20200920021344-4d4ec8eaf93c // indirect
	golang.org/x/net v0.47.0 // indirect
	golang.org/x/sys v0.38.0 // indirect
	golang.org/x/text v0.31.0 // indirect
)
