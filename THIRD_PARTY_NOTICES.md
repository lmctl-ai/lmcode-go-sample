# Runtime notices

The sample host is MIT licensed (see [LICENSE](LICENSE)). It links the Go
runtime SDK `github.com/lmctl-ai/lmcode-go` at the version pinned in `go.mod`;
the SDK carries the same MIT license text and copyright notice. The host has no
third-party Go module dependencies. CI installs lmscript only as a build and
corpus-verification tool; it is not included in the sample binary.
