package main

import (
	"context"
	"encoding/json"
	"flag"
	"os"
	"td-homework/examples/senior/releaseadmission"
)

func main() {
	root := flag.String("root", "", "dedicated source snapshot root")
	expected := flag.String("expected", "", "optional approved input digest")
	flag.Parse()
	d, err := releaseadmission.Fingerprint(context.Background(), *root)
	if err != nil {
		os.Exit(2)
	}
	match := *expected == "" || d == *expected
	if json.NewEncoder(os.Stdout).Encode(map[string]any{"digest": d, "matches": match}) != nil {
		os.Exit(2)
	}
	if !match {
		os.Exit(1)
	}
}
