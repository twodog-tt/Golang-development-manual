package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"

	"td-homework/examples/senior/agenteval"
)

func read(path string, dest any) {
	b, err := os.ReadFile(path)
	if err == nil {
		decoder := json.NewDecoder(bytes.NewReader(b))
		decoder.DisallowUnknownFields()
		err = decoder.Decode(dest)
		if err == nil {
			var extra any
			if decoder.Decode(&extra) != io.EOF {
				err = fmt.Errorf("%s: expected one JSON value", path)
			}
		}
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
}

func main() {
	suitePath := flag.String("suite", "examples/senior/agenteval/testdata/suite.json", "frozen suite")
	basePath := flag.String("baseline", "examples/senior/agenteval/testdata/baseline.json", "baseline artifact")
	candidatePath := flag.String("candidate", "examples/senior/agenteval/testdata/candidate-good.json", "candidate artifact")
	min := flag.Float64("min-success", 1, "minimum success ratio")
	regression := flag.Float64("max-regression", 0, "maximum absolute success ratio regression")
	flag.Parse()
	var suite agenteval.Suite
	var base, candidate agenteval.Artifact
	read(*suitePath, &suite)
	read(*basePath, &base)
	read(*candidatePath, &candidate)
	report, err := agenteval.Compare(suite, base, candidate, agenteval.Policy{MinSuccess: *min, MaxRegression: *regression})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(report); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	if !report.GatePassed {
		os.Exit(1)
	}
}
