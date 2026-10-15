package okta

import (
	"errors"
	"reflect"
	"strings"
	"testing"
)

const sampleJSON = `{"application_url":"https://example.okta.com/app","user":"alice","organization":"example.okta.com","accounts":[
 {"name":"team-a-dev","id":"111111111111","name_raw":"x","roles":[
   {"name":"arn:aws:iam::111111111111:role/Team-A-Ops","suffix":"Ops"},
   {"name":"arn:aws:iam::111111111111:role/Team-A-ReadOnly","suffix":"ReadOnly"}]},
 {"name":"team-a-private","id":"222222222222","name_raw":"y","roles":[
   {"name":"arn:aws:iam::222222222222:role/Team-A-Dev-Private-MgrAdmin","suffix":"MgrAdmin"},
   {"name":"arn:aws:iam::222222222222:role/Team-A-Dev-Private-PowerUser","suffix":"PowerUser"},
   {"name":"arn:aws:iam::222222222222:role/Team-A-Ops","suffix":"Ops"}]},
 {"name":"solo","id":"333333333333","name_raw":"z","roles":[
   {"name":"arn:aws:iam::333333333333:role/Solo-ReadOnly","suffix":"ReadOnly"}]}
]}`

func TestSplitCommand(t *testing.T) {
	tests := []struct {
		in      string
		want    []string
		wantErr bool
	}{
		{"a b  c", []string{"a", "b", "c"}, false},
		{`aws-okta-processor authenticate --user "j doe" --organization='x.okta.com'`,
			[]string{"aws-okta-processor", "authenticate", "--user", "j doe", "--organization=x.okta.com"}, false},
		{`cmd --empty=""`, []string{"cmd", "--empty="}, false},
		{`cmd "unterminated`, nil, true},
		{"", nil, false},
	}
	for _, tt := range tests {
		got, err := SplitCommand(tt.in)
		if (err != nil) != tt.wantErr {
			t.Errorf("SplitCommand(%q) err = %v, wantErr %v", tt.in, err, tt.wantErr)
			continue
		}
		if !tt.wantErr && !reflect.DeepEqual(got, tt.want) {
			t.Errorf("SplitCommand(%q) = %v, want %v", tt.in, got, tt.want)
		}
	}
}

func TestBuildGetRolesArgs(t *testing.T) {
	got, err := BuildGetRolesArgs("aws-okta-processor authenticate --user alice --organization example.okta.com", "yubico", "https://app")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"aws-okta-processor", "get-roles", "--user", "alice", "--organization", "example.okta.com", "--factor=yubico", "--application=https://app", "--output=json"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}

	if _, err := BuildGetRolesArgs("some-other-tool --flag", "", ""); err == nil {
		t.Error("expected error when there is no authenticate subcommand")
	}
}

func TestParseAccounts_ToleratesLeadingNoise(t *testing.T) {
	accounts, err := ParseAccounts([]byte("Info: something\n" + sampleJSON))
	if err != nil {
		t.Fatal(err)
	}
	if len(accounts) != 3 || accounts[1].ID != "222222222222" || len(accounts[1].Roles) != 3 {
		t.Errorf("unexpected accounts: %+v", accounts)
	}
	if _, err := ParseAccounts([]byte("no json here")); err == nil {
		t.Error("expected error without JSON")
	}
	if _, err := ParseAccounts([]byte("{broken")); err == nil {
		t.Error("expected error on malformed JSON")
	}
}

func TestPickRole(t *testing.T) {
	accounts, _ := ParseAccounts([]byte(sampleJSON))
	pref := []string{"PowerUser", "Ops", "ReadOnly"}

	tests := []struct {
		name      string
		accountID string
		pref      []string
		want      Choice
		found     bool
	}{
		{"first preference wins", "222222222222", pref, Choice{RoleARN: "arn:aws:iam::222222222222:role/Team-A-Dev-Private-PowerUser"}, true},
		{"falls to second preference", "111111111111", pref, Choice{RoleARN: "arn:aws:iam::111111111111:role/Team-A-Ops"}, true},
		{"case-insensitive", "111111111111", []string{"readonly"}, Choice{RoleARN: "arn:aws:iam::111111111111:role/Team-A-ReadOnly"}, true},
		{"single role needs no preference", "333333333333", nil, Choice{RoleARN: "arn:aws:iam::333333333333:role/Solo-ReadOnly"}, true},
		{"no match narrows prompt to account", "222222222222", []string{"Admin"}, Choice{AccountAlias: "team-a-private"}, true},
		{"no preference narrows prompt to account", "111111111111", nil, Choice{AccountAlias: "team-a-dev"}, true},
		{"unknown account", "999999999999", pref, Choice{}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, found := PickRole(accounts, tt.accountID, tt.pref)
			if found != tt.found || got != tt.want {
				t.Errorf("got (%+v, %v), want (%+v, %v)", got, found, tt.want, tt.found)
			}
		})
	}
}

func TestResolveRole(t *testing.T) {
	orig := runGetRoles
	defer func() { runGetRoles = orig }()

	var gotArgs []string
	runGetRoles = func(args []string) ([]byte, error) {
		gotArgs = args
		return []byte(sampleJSON), nil
	}

	choice, err := ResolveRole("aws-okta-processor authenticate --user alice", "yubico", "https://app", "222222222222", []string{"PowerUser"})
	if err != nil {
		t.Fatal(err)
	}
	if choice.RoleARN != "arn:aws:iam::222222222222:role/Team-A-Dev-Private-PowerUser" {
		t.Errorf("unexpected choice: %+v", choice)
	}
	if !strings.Contains(strings.Join(gotArgs, " "), "get-roles --user alice --factor=yubico --application=https://app --output=json") {
		t.Errorf("unexpected args: %v", gotArgs)
	}

	if _, err := ResolveRole("aws-okta-processor authenticate", "", "", "999999999999", nil); err == nil {
		t.Error("expected error for unknown account")
	}

	runGetRoles = func(args []string) ([]byte, error) { return nil, errors.New("boom") }
	if _, err := ResolveRole("aws-okta-processor authenticate", "", "", "222222222222", nil); err == nil {
		t.Error("expected error when get-roles fails")
	}

	if _, err := ResolveRole("not-okta --x", "", "", "222222222222", nil); err == nil {
		t.Error("expected error for non-authenticate credential_process")
	}
}
