package app

import (
	"fmt"
	"io"

	"github.com/D1ssolve/franzctl/internal/codec"
)

func runCodecs(args []string, stdout, stderr io.Writer) int {
	if len(args) != 0 {
		fmt.Fprintln(stderr, "usage: franzctl codecs")
		return 2
	}
	fmt.Fprintln(stdout, "Built-in codec stages:")
	for _, name := range codec.Builtins() {
		fmt.Fprintf(stdout, "  %s\n", name)
	}
	fmt.Fprintln(stdout, "\nCompose stages with commas. Encode runs left-to-right; decode runs right-to-left.")
	fmt.Fprintln(stdout, "Example: json,gzip")
	return 0
}
