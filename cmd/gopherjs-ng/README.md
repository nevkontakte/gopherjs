# GopherJS NG

**Current state:** early experiment, not viable for real use.
**Info:** This branch is currently based on the github.com/gopherjs/gopherjs repo.

This is an experimental GopherJS implementation that leans onto the upstream Go tool to drive the build process, and only invokes GopherJS for task directly related code generation. To achieve that, GopherJS implements CLI interfaces of compile/link/asm tools and injects itself into the build process using the `-toolexec` flag.

In theory, this approach has a number of advantages:

- New features implemented at the build toolchain level become available "for free" (e.g. modules support, file embedding, etc.).
- GopherJS can be easily integrated with other build systems that diverge from the GOPATH or Go Modules convention (e.g. Bazel).
- GopherJS only needs to maintain code generation logic and its standard library patches.
- Incremental and parallel builds "for free".

It also has a few potential disadvantages:

- Go compile/link/asm CLI is not a public interface strictly speaking, and may change between versions, as well as the intermediate file formats (.a/.o). It seems to be stable in practice, but there are no guarantees from the Go team.
- Currently several small patches need to be applied to the Go tool to make it work with GopherJS. It might be possible to upstream them, but that would require some goodwill on the Go team's part.

## Known issues

- The actual compilation is not implemented yet.
- `gopherjs-ng build` doesn't add `.js` extension to the file.
- `install/test/run` verbs are not supported.
- Doesn't support bundling raw '.js' files in.
- `serve` command is not supported.
- Build tags changed to GOOS=js GOARCH=js, in the past GOOS used to be build OS.
- No build tag unique to GopherJS.

## Setup

As mentioned above, currently, a few changes to the `go` tool are necessary in order for gopherjs-ng to function. Below is a short summary of steps that are required to build and user gopherjs-ng:

```
# Install target Go version:
$ GO_VERSION=go1.16
$ go get golang.org/dl/$GO_VERSION
$ $GO_VERSION download

# Set up some env variables for convenience:
$ export GO111MODULE=auto GOPATH="$($GO_VERSION env GOPATH)" GOROOT="$($GO_VERSION env GOROOT)"

# Apply go tool patches
$ cd "$($GO_VERSION env GOROOT)/src"
$ patch -p0 <<EOF
diff --color -r -u cmd/dist/build.go cmd/dist/build.go
--- cmd/dist/build.go	2021-01-31 20:15:25.710799648 +0000
+++ cmd/dist/build.go	2021-01-31 20:18:39.663458302 +0000
@@ -1559,6 +1559,7 @@
 	"ios/arm64":       true,
 	"ios/amd64":       true,
 	"js/wasm":         false,
+	"js/js":           false,
 	"netbsd/386":      true,
 	"netbsd/amd64":    true,
 	"netbsd/arm":      true,
diff --color -r -u cmd/go/internal/cfg/zosarch.go cmd/go/internal/cfg/zosarch.go
--- cmd/go/internal/cfg/zosarch.go	2021-01-31 20:15:25.714799660 +0000
+++ cmd/go/internal/cfg/zosarch.go	2021-01-31 20:20:30.615871311 +0000
@@ -19,6 +19,7 @@
 	"ios/amd64": true,
 	"ios/arm64": true,
 	"js/wasm": false,
+	"js/js": false,
 	"linux/386": true,
 	"linux/amd64": true,
 	"linux/arm": true,
diff --color -r -u cmd/go/internal/work/exec.go cmd/go/internal/work/exec.go
--- cmd/go/internal/work/exec.go	2021-01-31 20:15:25.726799698 +0000
+++ cmd/go/internal/work/exec.go	2021-01-31 20:31:35.166651850 +0000
@@ -1847,6 +1847,7 @@
 	{0x00, 0x61, 0x73, 0x6D},                  // WASM
 	{0x01, 0xDF},                              // XCOFF 32bit
 	{0x01, 0xF7},                              // XCOFF 64bit
+	[]byte("/*!gopherjs"),                     // GopherJS
 }
EOF
$ $GO_VERSION build -v -o ../bin/go cmd/go

# Get hacking
$ cd "$GOPATH/github.com/gopherjs/gopherjs"  # or wherever it is checked out.
$ $GO_VERSION install -v ./cmd/... && (cd "$GOPATH/src"; gopherjs-ng build -v -x some/package)
```
