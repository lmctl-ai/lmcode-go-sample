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

var version = "0.4.0"

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
		fmt.Println("lmcode-go-sample " + version)
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

	// The sink belongs to this invocation, not to a body/entry collection arena.
	var sink lmcode.ResultSink
	if err := host.Registry.SetResultSink(&sink); err != nil {
		return lmcode.ReportError("run", err)
	}
	context := lmcode.NewContext()
	context.Set("shop", host.Shop)
	context.Set("ledger", host.Ledger)
	if err := program.Run(context, host.Registry); err != nil {
		return lmcode.ReportError("run", err)
	}
	entries := program.Entries()
	entriesCalled := 0
	callEntry := func(index int, argument lmcode.Value) error {
		value, err := program.Call(index, context, host.Registry, []lmcode.Value{argument})
		if err != nil {
			return err
		}
		entriesCalled++
		if value.Kind != lmcode.KindVoid {
			text := value.Str
			switch value.Kind {
			case lmcode.KindInt:
				text = fmt.Sprintf("%d", value.Int)
			case lmcode.KindFloat:
				text = lmcode.FormatG15(value.Float)
			case lmcode.KindBool:
				text = fmt.Sprint(value.Bool)
			}
			fmt.Printf("entry %s = %s\n", entries[index].Name, text)
		}
		return nil
	}
	for index, entry := range entries {
		switch entry.Annotation {
		case "item":
			for _, item := range host.Shop.Items.Values {
				if err := callEntry(index, lmcode.Object("Money", item)); err != nil {
					return lmcode.ReportError("run", err)
				}
			}
		case "total":
			if err := callEntry(index, host.totalHandle(host.Shop)); err != nil {
				return lmcode.ReportError("run", err)
			}
		}
	}
	if host.Ledger.Balance < 0 {
		return lmcode.ReportAppFailure("ledger: negative balance")
	}
	result := fmt.Sprintf("{\"balance\": %d, \"entries\": %d, \"note\": %s, \"entries_called\": %d}",
		host.Ledger.Balance, host.Ledger.Entries, lmcode.JSONEscape(host.Ledger.Note), entriesCalled)
	if code := lmcode.ReportOK(result); code != 0 {
		return code
	}
	// Every error path above returns before the terminal success frame.
	if err := lmcode.WriteResultFrame(os.Stdout, &sink); err != nil {
		return lmcode.ReportAppFailure("cannot write sample result")
	}
	return 0
}

func main() { os.Exit(run(os.Args[1:])) }
