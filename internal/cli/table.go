package cli

import (
	"fmt"
	"io"
	"strings"
	"text/tabwriter"
)

func newTable(output io.Writer, headers ...string) *tabwriter.Writer {
	table := tabwriter.NewWriter(output, 0, 4, 2, ' ', 0)
	fmt.Fprintln(table, strings.Join(headers, "\t"))
	return table
}
