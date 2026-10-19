package main

import (
	"encoding/json"
	"io"
	"os"
	"td-homework/examples/senior/agentplatform"
)

func main() {
	b, e := io.ReadAll(io.LimitReader(os.Stdin, 8193))
	if e != nil {
		os.Exit(2)
	}
	c, h, e := agentplatform.Canonical(b)
	if e != nil {
		os.Exit(2)
	}
	if json.NewEncoder(os.Stdout).Encode(map[string]string{"canonical": c, "digest": h}) != nil {
		os.Exit(2)
	}
}
