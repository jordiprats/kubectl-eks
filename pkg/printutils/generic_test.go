package printutils

import (
	"io"
	"os"
	"strings"
	"testing"

	"github.com/jordiprats/kubectl-eks/pkg/data"
)

func TestFormatClusterNodeHealth(t *testing.T) {
	if got := formatClusterNodeHealth(0, 0, 0, 0); got != "-" {
		t.Fatalf("formatClusterNodeHealth() = %q, want %q", got, "-")
	}

	if got := formatClusterNodeHealth(10, 9, 1, 0); got != "9/10 Ready (NR:1 SD:0)" {
		t.Fatalf("formatClusterNodeHealth() = %q, want %q", got, "9/10 Ready (NR:1 SD:0)")
	}
}

func TestPrintCurrentClusterWide(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	orig := os.Stdout
	os.Stdout = w
	PrintCurrentClusterWide(false, "Team-A-PowerUser", data.ClusterInfo{
		AWSProfile: "dev", Region: "us-west-2", ClusterName: "c1", Namespace: "default",
		Status: "ACTIVE", Version: "1.35", Arn: "arn:aws:eks:us-west-2:111111111111:cluster/c1",
	})
	w.Close()
	os.Stdout = orig

	out, _ := io.ReadAll(r)
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected header and one row, got %q", out)
	}
	if !strings.Contains(lines[0], "IAM ROLE") {
		t.Errorf("header should contain IAM ROLE: %q", lines[0])
	}
	if strings.Index(lines[0], "IAM ROLE") > strings.Index(lines[0], "ARN") {
		t.Errorf("IAM ROLE should come before ARN: %q", lines[0])
	}
	if !strings.HasSuffix(strings.TrimSpace(lines[0]), "ARN") {
		t.Errorf("header should end with ARN: %q", lines[0])
	}
	if strings.Index(lines[1], "Team-A-PowerUser") > strings.Index(lines[1], "arn:aws:eks") {
		t.Errorf("role should come before the cluster ARN: %q", lines[1])
	}
}
