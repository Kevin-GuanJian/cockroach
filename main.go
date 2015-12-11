package main

import (
	"fmt"
	"math/rand"
	"os"

	"github.com/dmatrixdb/dmatrix/cli"
	"github.com/dmatrixdb/dmatrix/util/randutil"
)

func main() {
	// Seed the math/rand RNG from crypto/rand.
	rand.Seed(randutil.NewPseudoSeed())

	if len(os.Args) == 1 {
		os.Args = append(os.Args, "help")
	}
	if err := cli.Run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "Failed running command %q: %v\n", os.Args[1:], err)
		os.Exit(1)
	}
}
