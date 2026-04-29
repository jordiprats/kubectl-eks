package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsClusterScoped(t *testing.T) {
	clusterScoped := []string{
		"nodes",
		"namespaces",
		"persistentvolumes",
		"clusterroles",
		"clusterrolebindings",
		"storageclasses",
		"customresourcedefinitions",
		"priorityclasses",
	}
	for _, resource := range clusterScoped {
		t.Run(resource+" is cluster-scoped", func(t *testing.T) {
			assert.True(t, isClusterScoped(resource))
		})
	}

	namespacedResources := []string{
		"pods",
		"deployments",
		"services",
		"configmaps",
		"secrets",
		"statefulsets",
	}
	for _, resource := range namespacedResources {
		t.Run(resource+" is namespaced", func(t *testing.T) {
			assert.False(t, isClusterScoped(resource))
		})
	}
}

func TestFormatValue(t *testing.T) {
	tests := []struct {
		name     string
		input    interface{}
		expected string
	}{
		{
			name:     "string value",
			input:    "hello",
			expected: "hello",
		},
		{
			name:     "int value",
			input:    42,
			expected: "42",
		},
		{
			name:     "int64 value",
			input:    int64(100),
			expected: "100",
		},
		{
			name:     "float64 value",
			input:    3.14,
			expected: "3.14",
		},
		{
			name:     "bool true",
			input:    true,
			expected: "true",
		},
		{
			name:     "bool false",
			input:    false,
			expected: "false",
		},
		{
			name:     "slice value (JSON)",
			input:    []string{"a", "b"},
			expected: `["a","b"]`,
		},
		{
			name:     "map value (JSON)",
			input:    map[string]string{"key": "val"},
			expected: `{"key":"val"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatValue(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestMatchesFilter(t *testing.T) {
	obj := map[string]interface{}{
		"status": map[string]interface{}{
			"phase": "Running",
			"nodeInfo": map[string]interface{}{
				"kubeletVersion": "v1.29.0",
			},
			"readyReplicas": int64(3),
		},
		"metadata": map[string]interface{}{
			"name":      "my-pod",
			"namespace": "default",
		},
	}

	tests := []struct {
		name     string
		filter   string
		expected bool
	}{
		{"empty filter matches", "", true},
		{"top-level nested match", "status.phase=Running", true},
		{"top-level nested no match", "status.phase=Pending", false},
		{"deep nested match", "status.nodeInfo.kubeletVersion=v1.29.0", true},
		{"numeric value match", "status.readyReplicas=3", true},
		{"metadata match", "metadata.name=my-pod", true},
		{"nonexistent field", "status.missing=value", false},
		{"invalid path", "nonexistent.deep.path=x", false},
		{"no equals sign", "invalidfilter", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, matchesFilter(obj, tt.filter))
		})
	}
}
