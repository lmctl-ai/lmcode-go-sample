// lmcode-go-sample is a native bytecode host. lmscript compiles programs;
// this process only loads the pinned bytes and runs them against its registry.
package main

import (
	"embed"
	"errors"
	"fmt"
	"os"

	"github.com/lmctl-ai/lmcode-go"
)

//go:embed sample-suite/sample.dsl.json
var schemaBytes embed.FS

const usage = "usage: --version | --dsl-schema [--json] | --dsl-signatures | --dsl FILE"

func cliUsage() int {
	fmt.Printf("{\"ok\": false, \"phase\": \"cli\", \"code\": \"CLI_USAGE\", \"message\": %s}\n", lmcode.JSONEscape(usage))
	return 2
}

func normalizedLoadError(err error) error {
	var sdkErr *lmcode.Error
	if !errors.As(err, &sdkErr) {
		return lmcode.NewError(lmcode.LOAD_BAD_BYTECODE, "invalid sample bytecode")
	}
	message := "invalid sample bytecode"
	switch sdkErr.Code {
	case lmcode.LOAD_SCHEMA_MISMATCH:
		message = "sample schema mismatch"
	case lmcode.LOAD_UNBOUND:
		message = "sample import mismatch"
	}
	return lmcode.NewError(sdkErr.Code, message)
}

func run(args []string) int {
	host := NewHost()

	if len(args) == 1 && args[0] == "--version" {
		fmt.Println("lmcode-go-sample 0.1.0")
		return 0
	}
	if len(args) == 1 && args[0] == "--dsl-schema" ||
		len(args) == 2 && args[0] == "--dsl-schema" && args[1] == "--json" {
		data, err := schemaBytes.ReadFile("sample-suite/sample.dsl.json")
		if err != nil {
			return lmcode.ReportAppFailure("cannot read sample schema")
		}
		_, _ = os.Stdout.Write(data) // Exact embedded bytes, including the final LF.
		return 0
	}
	if len(args) == 1 && args[0] == "--dsl-signatures" {
		for _, signature := range host.Registry.Signatures() {
			fmt.Fprintln(os.Stdout, signature)
		}
		return 0
	}
	if len(args) != 2 || args[0] != "--dsl" {
		return cliUsage()
	}

	bytecode, err := os.ReadFile(args[1])
	if err != nil {
		return lmcode.ReportAppFailure("cannot read sample input")
	}
	schema, err := schemaBytes.ReadFile("sample-suite/sample.dsl.json")
	if err != nil {
		return lmcode.ReportAppFailure("cannot read sample schema")
	}
	program, err := lmcode.Load(bytecode, schema, host.Registry)
	if err != nil {
		return lmcode.ReportError("load", normalizedLoadError(err))
	}

	context := lmcode.NewContext()
	context.Set("shop", host.Shop)
	context.Set("ledger", host.Ledger)
	if err := program.Run(context, host.Registry); err != nil {
		return lmcode.ReportError("run", err)
	}
	if host.Ledger.Balance < 0 {
		return lmcode.ReportAppFailure("ledger: negative balance")
	}
	result := fmt.Sprintf("{\"balance\": %d, \"entries\": %d, \"note\": %s}",
		host.Ledger.Balance, host.Ledger.Entries, lmcode.JSONEscape(host.Ledger.Note))
	return lmcode.ReportOK(result)
}

func main() { os.Exit(run(os.Args[1:])) }
