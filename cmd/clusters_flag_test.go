package cmd

import (
	"testing"

	"github.com/spf13/cobra"
)

func TestNodeCountFlagAvailableOnListAndNodes(t *testing.T) {
	tests := []struct {
		name    string
		command *cobra.Command
	}{
		{name: "list", command: listCmd},
		{name: "nodes", command: nodesCmd},
	}

	for _, test := range tests {
		flag := test.command.Flags().Lookup("node-count")
		if flag == nil {
			t.Fatalf("%s command does not define --node-count", test.name)
		}
		if flag.Shorthand != "C" {
			t.Errorf("%s --node-count shorthand = %q, want %q", test.name, flag.Shorthand, "C")
		}
	}
}
