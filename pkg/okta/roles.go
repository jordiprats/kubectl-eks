package okta

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"
)

type Role struct {
	Name   string `json:"name"`
	Suffix string `json:"suffix"`
}

type Account struct {
	Name  string `json:"name"`
	ID    string `json:"id"`
	Roles []Role `json:"roles"`
}

type Choice struct {
	RoleARN      string
	AccountAlias string
}

var runGetRoles = func(args []string) ([]byte, error) {
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Stdin = os.Stdin
	cmd.Stderr = os.Stderr
	return cmd.Output()
}

// HasValidCredentials runs a credential_process without a controlling
// terminal and with stdin closed, so it can only succeed from its cache: any
// login or role prompt fails instead of interrupting the user.
func HasValidCredentials(args []string) bool {
	if len(args) == 0 {
		return false
	}
	cmd := exec.Command(args[0], args[1:]...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	out, err := cmd.Output()
	if err != nil {
		return false
	}
	var creds struct {
		Expiration string `json:"Expiration"`
	}
	start := bytes.IndexByte(out, '{')
	if start < 0 || json.Unmarshal(out[start:], &creds) != nil {
		return false
	}
	exp, err := time.Parse(time.RFC3339, creds.Expiration)
	if err != nil {
		return false
	}
	return time.Until(exp) > time.Minute
}

func SplitCommand(s string) ([]string, error) {
	var args []string
	var cur strings.Builder
	var quote rune
	inToken := false

	for _, r := range s {
		switch {
		case quote != 0:
			if r == quote {
				quote = 0
			} else {
				cur.WriteRune(r)
			}
		case r == '"' || r == '\'':
			quote = r
			inToken = true
		case r == ' ' || r == '\t':
			if inToken {
				args = append(args, cur.String())
				cur.Reset()
				inToken = false
			}
		default:
			cur.WriteRune(r)
			inToken = true
		}
	}
	if quote != 0 {
		return nil, fmt.Errorf("unterminated quote in %q", s)
	}
	if inToken {
		args = append(args, cur.String())
	}
	return args, nil
}

func BuildGetRolesArgs(credentialProcess, factor, application string) ([]string, error) {
	args, err := SplitCommand(credentialProcess)
	if err != nil {
		return nil, err
	}
	for i, a := range args {
		if a == "authenticate" {
			args[i] = "get-roles"
			if factor != "" {
				args = append(args, "--factor="+factor)
			}
			if application != "" {
				args = append(args, "--application="+application)
			}
			return append(args, "--output=json"), nil
		}
	}
	return nil, fmt.Errorf("credential_process has no 'authenticate' subcommand: %q", credentialProcess)
}

func ParseAccounts(out []byte) ([]Account, error) {
	start := bytes.IndexByte(out, '{')
	if start < 0 {
		return nil, fmt.Errorf("no JSON found in get-roles output")
	}
	var result struct {
		Accounts []Account `json:"accounts"`
	}
	if err := json.Unmarshal(out[start:], &result); err != nil {
		return nil, fmt.Errorf("failed to parse get-roles output: %w", err)
	}
	return result.Accounts, nil
}

func roleSuffix(arn string) string {
	name := arn[strings.LastIndex(arn, "/")+1:]
	return name[strings.LastIndex(name, "-")+1:]
}

func PickRole(accounts []Account, accountID string, preference []string) (Choice, bool) {
	for _, acc := range accounts {
		if acc.ID != accountID {
			continue
		}
		for _, pref := range preference {
			for _, r := range acc.Roles {
				if strings.EqualFold(roleSuffix(r.Name), pref) {
					return Choice{RoleARN: r.Name}, true
				}
			}
		}
		if len(acc.Roles) == 1 {
			return Choice{RoleARN: acc.Roles[0].Name}, true
		}
		return Choice{AccountAlias: acc.Name}, true
	}
	return Choice{}, false
}

func ResolveRole(credentialProcess, factor, application, accountID string, preference []string) (Choice, error) {
	args, err := BuildGetRolesArgs(credentialProcess, factor, application)
	if err != nil {
		return Choice{}, err
	}
	out, err := runGetRoles(args)
	if err != nil {
		return Choice{}, fmt.Errorf("get-roles failed: %w", err)
	}
	accounts, err := ParseAccounts(out)
	if err != nil {
		return Choice{}, err
	}
	choice, found := PickRole(accounts, accountID, preference)
	if !found {
		return Choice{}, fmt.Errorf("account %s not available in this Okta application", accountID)
	}
	return choice, nil
}
