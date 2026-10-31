// lmcode-go-sample is a native bytecode host. lmscript compiles programs;
// this process loads and runs each submitted program in one shared host.
package main

import (
	"embed"
	"errors"
	"fmt"
	"os"
	"sort"

	"github.com/lmctl-ai/lmcode-go"
)

//go:embed sample-suite/sample.dsl.json
var schemaBytes embed.FS

var version = "0.4.0"

const usage = "usage: --version | --lmcode-schema [--json] | --lmcode-signatures | [--lmcode|--first|--last] FILE ..."

type invocationArg struct {
	file     string
	priority int
	index    int
}

func cliFailure(err error) int { return lmcode.FailureFrame(os.Stdout, "cli", err) }

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
	if len(args) == 1 && args[0] == "--lmcode-schema" || len(args) == 2 && args[0] == "--lmcode-schema" && args[1] == "--json" {
		data, err := schemaBytes.ReadFile("sample-suite/sample.dsl.json")
		if err != nil {
			return lmcode.FailureFrame(os.Stdout, "app", lmcode.NewError(lmcode.APP_FAIL, "cannot read sample schema"))
		}
		_, _ = os.Stdout.Write(data)
		return 0
	}
	if len(args) == 1 && args[0] == "--lmcode-signatures" {
		for _, signature := range host.Registry.Signatures() {
			fmt.Fprintln(os.Stdout, signature)
		}
		return 0
	}

	parts := make([]invocationArg, 0, len(args)/2)
	files := make([]string, 0, len(args)/2)
	for i := 0; i < len(args); i += 2 {
		if i+1 >= len(args) {
			return cliFailure(lmcode.NewError(lmcode.CLI_USAGE, usage))
		}
		priority := 1
		switch args[i] {
		case "--lmcode":
		case "--first":
			priority = 0
		case "--last":
			priority = 2
		default:
			return cliFailure(lmcode.NewError(lmcode.CLI_USAGE, usage))
		}
		parts = append(parts, invocationArg{file: args[i+1], priority: priority, index: len(parts)})
		files = append(files, args[i+1])
	}
	if len(parts) == 0 {
		return cliFailure(lmcode.NewError(lmcode.CLI_USAGE, usage))
	}
	results, err := lmcode.InvocationResultsFromCommand(files)
	if err != nil {
		return cliFailure(err)
	}
	schema, err := schemaBytes.ReadFile("sample-suite/sample.dsl.json")
	if err != nil {
		return lmcode.FailureFrame(os.Stdout, "load", lmcode.NewError(lmcode.LOAD_BAD_BYTECODE, "cannot read sample schema"))
	}
	contract, err := lmcode.ResultContractFromSchema(schema)
	if err != nil {
		return lmcode.FailureFrame(os.Stdout, "load", err)
	}
	context := lmcode.NewContext()
	context.Set("shop", host.Shop)
	context.Set("ledger", host.Ledger)
	entriesCalled := 0
	sort.SliceStable(parts, func(i, j int) bool { return parts[i].priority < parts[j].priority })
	failed := false
	for _, part := range parts {
		phase := "load"
		bytecode, readErr := os.ReadFile(part.file)
		if readErr != nil {
			_ = results.Fail(&part.index, "app", lmcode.NewError(lmcode.APP_FAIL, "cannot read sample input"))
			failed = true
			break
		}
		program, loadErr := lmcode.Load(bytecode, schema, host.Registry)
		if loadErr != nil {
			_ = results.Fail(&part.index, phase, normalizedLoadError(loadErr))
			failed = true
			break
		}
		if _, err = results.Select(part.index, host.Registry, contract); err != nil {
			_ = results.Fail(&part.index, "run", err)
			failed = true
			break
		}
		phase = "run"
		if err = program.Run(context, host.Registry); err != nil {
			_ = results.Fail(&part.index, phase, err)
			failed = true
			break
		}
		entries := program.Entries()
		callEntry := func(index int, argument lmcode.Value) error {
			value, callErr := program.Call(index, context, host.Registry, []lmcode.Value{argument})
			if callErr != nil {
				return callErr
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
					if err = callEntry(index, lmcode.Object("Money", item)); err != nil {
						break
					}
				}
			case "total":
				err = callEntry(index, host.totalHandle(host.Shop))
			}
			if err != nil {
				break
			}
		}
		if err != nil {
			_ = results.Fail(&part.index, "run", err)
			failed = true
			break
		}
		if err = results.Complete(part.index); err != nil {
			_ = results.Fail(&part.index, "run", err)
			failed = true
			break
		}
	}
	if !failed && host.Ledger.Balance < 0 {
		_ = results.Fail(nil, "app", lmcode.NewError(lmcode.APP_FAIL, "ledger: negative balance"))
		failed = true
	}
	if !failed {
		fmt.Printf("{\"host_summary\":{\"balance\": %d, \"entries\": %d, \"note\": %s, \"entries_called\": %d}}\n", host.Ledger.Balance, host.Ledger.Entries, lmcode.JSONEscape(host.Ledger.Note), entriesCalled)
	}
	return results.WriteFrame(os.Stdout)
}

func main() { os.Exit(run(os.Args[1:])) }
