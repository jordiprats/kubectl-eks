package sts

import "testing"

func TestRoleFromCallerARN(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"arn:aws:sts::111111111111:assumed-role/Team-A-PowerUser/alice@example.com", "Team-A-PowerUser"},
		{"arn:aws:sts::111111111111:assumed-role/Team-A-Ops/session-1", "Team-A-Ops"},
		{"arn:aws:iam::111111111111:user/alice", "user/alice"},
		{"arn:aws:iam::111111111111:root", "root"},
		{"arn:aws:sts::111111111111:federated-user/bob", "federated-user/bob"},
		{"not-an-arn", "not-an-arn"},
		{"", ""},
	}
	for _, tt := range tests {
		if got := RoleFromCallerARN(tt.in); got != tt.want {
			t.Errorf("RoleFromCallerARN(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}
