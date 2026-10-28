# lmcode Go shop sample

**Agent entry point:** this is a complete Go host for lmcode bytecode, not an
interpreter. Start with [host.go](host.go) for domain objects and registry
callbacks, then [main.go](main.go) for the CLI protocol. It uses the public
`github.com/lmctl-ai/lmcode-go` module with CGO disabled and embeds the exact
schema bytes from the pinned suite.

```sh
npm install --global @lmctl-ai/lmctl@0.2.426
./test.sh
```

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
   DSL handle stays valid for the process lifetime. `Items.at()` returns the
   pointer already stored in the shop, preserving identity.
5. **Register contexts and typed callbacks.** `Registry.Context`, `Function`,
   and `Method` declare the host surface with schema type names. Parameter
   tuples distinguish overloads, and callbacks return tagged `lmcode.Value`s.
6. **Map DSL operations explicitly.** `Money` operators call `plus`, `minus`,
   `times`, and `neg`; `money(int)` means cents while `money(float)` means
   currency units. Signed integer money arithmetic wraps modulo 2^64.
7. **Use standard libraries deliberately.** `lmcode.NewRegistry()` installs
   string, math, file, collection, container, and JSON functions. The host separately calls
   `lmcode.RegisterTesting` to opt into `print` and `assert_close`. Collection
   handles created by std functions are owned by the registry for one run.
8. **Load bytecode, not DSL source.** The compiler builds `.dslb`; [main.go](main.go)
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
| `float` | `float64` | Stay in binary64; use `lmcode.FormatG15` for stable text, and `math.Round` for ties-away currency rounding. |
| `bool` | `bool` | Map directly to `lmcode.Bool`. |
| `string` | `string` / `[]byte` | Strings are raw byte strings, not Unicode text; use byte indexing and do not normalize UTF-8. |
| object handle | typed pointer, such as `*Money` | Keep returned domain objects in the `Host` arena; preserve pointer identity. |
| standard collection | `*stdlib.StrList`, `*stdlib.IntList`, `*stdlib.FloatList`, `*stdlib.StrMap`, typed maps, stacks, queues, PQs, `*stdlib.JSON` | Read snapshots with `Values()`/`Entries()`; std handles live in the registry arena for one `Program.Run`. |
| enum | string | Return the declared value, for example `"USD"` for `Currency.USD`. |
| `void` | `lmcode.None()` | Host callbacks return `None` with no error. |
| host error | `*lmcode.Error` | Use `lmcode.NewHostError`; the VM adds the DSL source location. |
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
oracle. The current suite passes **91/91 cases** and covers all **181 standard
signatures** and **45 host signatures**; expected outputs are pinned and are
never regenerated by tests.
The Go SDK is pinned by `go.mod`/`go.sum`; its MIT terms are recorded in
[THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md).

## Build and sync

```sh
CGO_ENABLED=0 go build -trimpath -o build/lmcode-go-sample .
./scripts/sync-suite.sh /path/to/clean/lmcode-cpp
./test.sh
```

CI runs Linux, macOS, and Windows. Its manual workflow requires a published
R47/R48-capable compiler version input. Locally, use
`LMSCRIPT_BIN=/path/to/current/lmscript LMCODE_REQUIRE_LITERAL_COMPILER=1 ./test.sh`
to reproduce all 91 pinned programs. The runtime binary itself is pure Go and
built with `CGO_ENABLED=0`.

## Rule methods (R47/R48)

Suite commit `aec9b09` adds program 067: the chunk calls an `@PRICE_RULE`
function implementing schema SAM interface `Adjuster` to update prices.
The shared compile-error fixture rejects a mismatched rule signature.
Annotations and interface bindings are compiler-only; no runtime dispatch or
SDK update is needed. Bytecode remains DSLB v5, but the interface-bearing schema
has a new hash, so the suite pins recompiled programs against those exact bytes.
