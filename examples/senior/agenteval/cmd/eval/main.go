package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"

	"td-homework/examples/senior/agenteval"
)

func read(path string, dest any) error {
	b, err := os.ReadFile(path)
	if err == nil {
		err = agenteval.DecodeJSON(b, dest)
	}
	if err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	return nil
}

func run(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("eval", flag.ContinueOnError)
	flags.SetOutput(stderr)
	suitePath := flags.String("suite", "examples/senior/agenteval/testdata/suite.json", "frozen suite")
	basePath := flags.String("baseline", "examples/senior/agenteval/testdata/baseline.json", "baseline artifact")
	candidatePath := flags.String("candidate", "examples/senior/agenteval/testdata/candidate-good.json", "candidate artifact")
	min := flags.Float64("min-success", 1, "minimum success ratio")
	regression := flags.Float64("max-regression", 0, "maximum absolute success ratio regression")
	hashOnly := flags.Bool("suite-hash", false, "print validated suite SHA-256 without scoring")
	if err := flags.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		return 2
	}
	if flags.NArg() != 0 {
		fmt.Fprintln(stderr, "unexpected positional arguments")
		return 2
	}
	var suite agenteval.Suite
	if err := read(*suitePath, &suite); err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	if *hashOnly {
		digest, err := agenteval.SuiteSHA256(suite)
		if err == nil {
			_, err = fmt.Fprintln(stdout, digest)
		}
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 2
		}
		return 0
	}
	var base, candidate agenteval.Artifact
	if err := read(*basePath, &base); err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	if err := read(*candidatePath, &candidate); err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	report, err := agenteval.Compare(suite, base, candidate, agenteval.Policy{MinSuccess: *min, MaxRegression: *regression})
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	encoder := json.NewEncoder(stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(report); err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	if !report.GatePassed {
		return 1
	}
	return 0
}

func main() { os.Exit(run(os.Args[1:], os.Stdout, os.Stderr)) }
