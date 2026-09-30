package cmd

import (
	"testing"
	"time"

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

func TestValidateListOutputOptions(t *testing.T) {
	tests := []struct {
		name      string
		output    string
		nodeCount bool
		arnOnly   bool
		nameOnly  bool
		wantError bool
	}{
		{name: "default"},
		{name: "wide", output: "wide"},
		{name: "node count", nodeCount: true},
		{name: "unknown output", output: "side", wantError: true},
		{name: "wide with node count", output: "wide", nodeCount: true, wantError: true},
		{name: "both scalar modes", arnOnly: true, nameOnly: true, wantError: true},
		{name: "wide scalar mode", output: "wide", arnOnly: true, wantError: true},
		{name: "node count scalar mode", nodeCount: true, nameOnly: true, wantError: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := validateListOutputOptions(test.output, test.nodeCount, test.arnOnly, test.nameOnly)
			if (err != nil) != test.wantError {
				t.Fatalf("validateListOutputOptions() error = %v, wantError %v", err, test.wantError)
			}
		})
	}
}

func TestValidateNodesOutputOptions(t *testing.T) {
	tests := []struct {
		name      string
		output    string
		nodeCount bool
		managedBy string
		older     string
		watch     time.Duration
		wantError bool
	}{
		{name: "default"},
		{name: "wide", output: "wide"},
		{name: "node count", nodeCount: true},
		{name: "unknown output", output: "side", wantError: true},
		{name: "wide with node count", output: "wide", nodeCount: true, wantError: true},
		{name: "managed by with node count", nodeCount: true, managedBy: "karpenter", wantError: true},
		{name: "older with node count", nodeCount: true, older: "1d", wantError: true},
		{name: "watch with node count", nodeCount: true, watch: time.Second, wantError: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := validateNodesOutputOptions(test.output, test.nodeCount, test.managedBy, test.older, test.watch)
			if (err != nil) != test.wantError {
				t.Fatalf("validateNodesOutputOptions() error = %v, wantError %v", err, test.wantError)
			}
		})
	}
}
