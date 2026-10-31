# lmcode Go shop sample

**Agent entry point:** this is a complete Go host for lmcode bytecode, not an
interpreter. Start with [host.go](host.go) for domain objects and registry
callbacks, then [main.go](main.go) for the CLI protocol. It uses the public
`github.com/lmctl-ai/lmcode-go` module with CGO disabled and embeds the exact
schema bytes from the pinned suite.

## Prerequisites

Use Go 1.26, Python 3, Node 24 and Bash. Build lmscript from mono main
**177e38ca or later** and set `LMSCRIPT_BIN` to its `lmscript/bin/lmscript`.
Published npm compiler 0.2.448 lacks `out=`; source builds may report 0.2.38,
so source SHA and successful corpus reproduction establish capability.
Run `./test.sh` with that compiler. No global install is required.


## Write a host in ten steps

1. **Pick a small domain.** This host models `Money`, `Shop`, `Items`, and
   `Ledger`; the portable behavior is specified in
   [CONTRACT.md](sample-suite/CONTRACT.md).
2. **Declare the API.** [host.schema.json](sample-suite/host.schema.json) gives
   contexts, object types, overloads, operators, an enum, and return-self
   methods. [sample.dsl.json](sample-suite/sample.dsl.json) adds the standard
   catalog used by the compiler.
3. **Pin the shared corpus.** [suite.lock](suite.lock) hashes every vendored
   file. [scripts/sync-suite.sh](scripts/sync-suite.sh) copies only from a clean,
   reviewed commit of the canonical `lmcode-cpp` suite; the corpus remains
   canonical there, never in this sample.
4. **Own host objects.** [Host](host.go) retains every `*Money` it returns so a
   lmcode handle stays valid for the process lifetime. `Items.at()` returns the
   pointer already stored in the shop, preserving identity.
5. **Register contexts and typed callbacks.** `Registry.Context`, `Function`,
   and `Method` declare the host surface with schema type names. Parameter
   tuples distinguish overloads, and callbacks return tagged `lmcode.Value`s.
6. **Map lmcode operations explicitly.** `Money` operators call `plus`, `minus`,
   `times`, and `neg`; `money(int)` means cents while `money(float)` means
   currency units. Signed integer money arithmetic wraps modulo 2^64.
7. **Use standard libraries deliberately.** `lmcode.NewRegistry()` installs
   string, math, file, collection, container, and JSON functions. The host separately calls
   `lmcode.RegisterTesting` to opt into `print` and `assert_close`. Collection
   handles created by std functions are owned by the registry for one run.
8. **Load bytecode, not lmcode source.** The compiler builds `.dslb`; [main.go](main.go)
   loads it against the embedded, exact schema bytes with `lmcode.Load`, then
   runs it with a `Context` holding the live shop and ledger.
9. **Keep the CLI protocol deterministic.** Discovery flags write the embedded
   schema or sorted registry signatures. Load/run failures use the SDK's JSON
   reporters; loader messages are normalized to the portable contract, and a
   negative ledger balance is an application error (exit 4).
10. **Run the integration suite.** [test.sh](test.sh) builds one static host,
    checks registry coverage and compiler reproducibility, validates CLI
    discovery, compares exact stdout/exit codes for all cases, and runs
    negative controls for the test harness.

## Go and lmcode values

| lmcode type | Go representation | Boundary rule |
|---|---|---|
| `int` | `int64` | Use `uint64` intermediates for explicit two's-complement wrapping. |
| `float` | `float64` | Stay in binary64; use `lmcode.FormatG15` for VM trace text (return frames use shortest-round-trip formatting), and `math.Round` for ties-away currency rounding. |
| `bool` | `bool` | Map directly to `lmcode.Bool`. |
| `string` | `string` / `[]byte` | Strings are raw byte strings, not Unicode text; use byte indexing and do not normalize UTF-8. |
| object handle | typed pointer, such as `*Money` | Keep returned domain objects in the `Host` arena; preserve pointer identity. |
| standard collection | `*stdlib.StrList`, `*stdlib.IntList`, `*stdlib.FloatList`, `*stdlib.StrMap`, typed maps, stacks, queues, PQs, `*stdlib.JSON` | Read snapshots with `Values()`/`Entries()`; std handles live in the registry arena for one `Program.Run`. |
| enum | string | Return the declared value, for example `"USD"` for `Currency.USD`. |
| `void` | `lmcode.None()` | Host callbacks return `None` with no error. |
| host error | `*lmcode.Error` | Use `lmcode.NewHostError`; the VM adds the lmcode source location. |
| JSON string | `lmcode.JSONEscape` | Escapes a byte string for the result line without adding machine-specific paths. |

Go strings preserve arbitrary bytes, including NUL and invalid UTF-8. A Go
`range` loop decodes Unicode code points, so byte-oriented adapters should use
`[]byte(s)` or index `s[i]` instead. The SDK's `Value` has explicit tagged fields
(`Int`, `Float`, `Str`, `Bool`, `Obj`, and `Type`); callbacks should use the field
matching their declared signature.

## Feature coverage

| Feature | Programs |
|---|---|
| Literals, arithmetic, comparisons, logic, conditionals, loops, functions, switches | 001–008, 036, 039, 043, 045 |
| Annotated SAM rule called from the chunk | 067 |
| Host-dispatched item/total rules and returned scores | 068–069 |
| Invocation returns, owned copies, entry lifetime and failure suppression | 360–368 |
| v5 bytecode refused by the v6 runtime | 093 |
| Host overloads, objects, operators, conversion, chaining, and void methods | 009–014, 034–035, 040 |
| All standard collection methods | 015–018, 037–038 |
| Standard math overloads and all 18 math opcodes | 019–020, 025, 048 |
| Standard string functions and conversions | 021–023 |
| Standard file functions | 024 |
| Located VM/host errors, assertion failures, and application errors | 026–033 |
| int64 wrap, short-circuit behavior, and host foreach | 041–044 |
| Raw/dedented heredocs and outer splice types | 046–047 |
| Bad magic and schema-binding rejection | 090–091 |
| Native value/byte/handle boundaries and extended host imports | 049–066 |
| Typed maps, stacks, queues, PQs, and JSON parsing/navigation/errors | 101–108 |
| Strict byte codecs and independent map snapshots | 301–305, 311–313 |

The [canonical feature matrix](sample-suite/README.md) links each bytecode and
oracle. The current suite passes **123/123 cases** and covers all **206 standard
signatures** and **45 host signatures**; expected outputs are pinned and are
never regenerated by tests.
The Go SDK is pinned by `go.mod`/`go.sum`; its MIT terms are recorded in
[THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md).

## Build and sync

Build with `CGO_ENABLED=0 go build -trimpath -o build/lmcode-go-sample .`.
Sync a reviewed canonical checkout using `scripts/sync-suite.sh SOURCE_REPO`.
The SDK uses the repository's existing Go-module pin: update with
`go get github.com/lmctl-ai/lmcode-go@REVIEWED_COMMIT`, then `go mod tidy`
and `go mod verify`. Do not add a competing vendor or sdk.lock mechanism.
Run `./test.sh` after setting `LMSCRIPT_BIN` as above.


## lmscript tests and automatic releases

Use the prerequisites and selected compiler above. `test.sh` forwards to `test.lms`. Existing Actions compiler defaults need a compatible publication before the adopted corpus can qualify; no future npm version is assumed.

Every main push builds with CGO_ENABLED=0 and tests on Linux x64/arm64, macOS x64/arm64 and Windows x64, then publishes native executables, licenses and SHA256SUMS. Pull requests run the same checks without publishing.

`examples/rule-entries.lms` demonstrates annotated functions implementing Adjuster. After running the body the host calls @item once per original item and @total for the total, ignoring other annotations. Case 069 covers returned values. The lmscript driver audits all 206 standard and 45 host signatures, runs all 123 cases with exact bytes/exits, reproduces bytecode and checks nine negative runner controls plus the annotated example. Python verifies/synchronizes suite locks, checks shared frame oracles and compiles host mutation controls; Node provides binary/process adapters. The SDK remains pinned by go.mod/go.sum.

## Text templates

[examples/text-template.lms](examples/text-template.lms) renders ordinary text from a nested context with indexed lists, loops and boolean conditions. The standard `template_render(string, Json) -> string` function performs one pass; inserted values stay literal. Compile it with `$LMSCRIPT_BIN --no-preflight examples/text-template.lms build/text-template.dslb`, then run the sample host with `--dsl build/text-template.dslb`.

The lmscript-driven suite runs 123 cases, including template success/error cases. See the [portable template contract](https://github.com/lmctl-ai/lmcode-cpp/blob/main/std/template.md).

## Return protocol

One invocation owns a `lmcode.ResultSink`, configured with
`Registry.SetResultSink` before the body and retained across selected entries.
The sink is separate from per-region collection arenas. After dispatch and
negative-balance validation, the CLI keeps its existing shop diagnostic and calls
`lmcode.WriteResultFrame`. Unset produces null. Load, body, entry and application
errors return before framing; entry HALT values remain independent of `$return`.

### Program

For a source file in `examples/`:

```lmscript
program = _lmcode_ schema="../sample-suite/sample.dsl.json" out=ARGV[0];
$return.name = "shop";
$return.total = shop.total().amount();
end _lmcode_
```

### Expected result

```text
{"ok": true, "result": {"balance": 0, "entries": 0, "note": "", "entries_called": 0}}
LMCODE_RETURN_BEGIN
{"name":"shop","total":750}
LMCODE_RETURN_END
```

### Validation

The full gate includes ten shared Python/Node frame controls and actual compiled
host faults from `scripts/test-return-controls.py`. Removing the frame or resetting
the sink inside entry dispatch must fail canonical payload expectations. A compile
failure, stderr or wrong exit is not accepted as a caught mutant. See
[durable notes](durable-memory/index.md) for ownership and pin discipline.

## Historical literal names

The documentation calls the language lmcode. `--dsl`, `--dsl-schema` and `--dsl-signatures` are current host flag literals; `DSLB` is the bytecode magic and `DSLB v6` is the current format name; `.dslb` is the current bytecode filename extension. Other code-formatted API names, paths, manifest names and diagnostic text retain their current literal spellings.
