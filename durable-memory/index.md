# Go sample — return-frame adoption

## Pins

Suite: suite.lock is authoritative for the reviewed canonical commit and file
hashes (123 cases). Source, bytecode and oracle bytes retain the full-gate
qualification; documentation-only upstream updates are recorded by the lock.
SDK: lmcode-go 6fd8391ee4dd, pinned by go.mod/go.sum as
v0.6.39-0.20261031051649-6fd8391ee4dd. There is no separate SDK sync script or sdk.lock.
Compiler qualification: private mono main build 177e38ca. npm 0.2.448 lacks out=.

## Ownership and protocol

main.go creates one ResultSink and binds it through Registry.SetResultSink
before Program.Run. Keep that sink through the item/total entry calls; do not
reset it inside callEntry. After application validation, ReportOK preserves the
shop diagnostic and WriteResultFrame emits the final payload. Errors return
before framing. Case 364 verifies body/entry ownership; 365–368 frame no failures.

## Gates

./test.sh uses LMSCRIPT_BIN, verifies the canonical lock and Go module, audits
206 std/45 host imports, runs 123 exact cases and reproduces their bytecode,
exercises runner controls and both local examples. Shared frame oracles and
compiled missing-frame/reset-between-entries controls run in the same gate.
Canonical expectations are never recorded from this port. Keep suite updates
in lmcode-cpp and sync committed bytes with scripts/sync-suite.py.

## Platform and CI

Local qualification is Linux x86_64 with CGO_ENABLED=0. The existing Actions
platform matrix and automatic release policy are unchanged. Its npm compiler
default requires a compatible publication before the new corpus can pass there;
do not invent a future package version. SAMPLE_VERSION remains the linker-set
version label, independent of the invocation return value.
