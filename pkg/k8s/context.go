package k8s

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"k8s.io/client-go/tools/clientcmd"
)

func SetNamespace(namespace string) error {
	cmd := exec.Command("kubectl", "config", "set-context", "--current", "--namespace", namespace)
	cmd.Stdout = nil
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func GetCurrentNamespace() (string, error) {
	cmd := exec.Command("kubectl", "config", "view", "--minify", "-o", "jsonpath='{.contexts[0].context.namespace}'")
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}

	// Remove surrounding quotes and trim whitespace
	result := strings.Trim(strings.TrimSpace(string(output)), "'\"")
	return result, nil
}

// FindContextForCluster checks if a kubeconfig context already exists for the
// given cluster ARN.
// Returns the context name and true when a matching context is found.
// It intentionally does NOT make any API calls to validate the credentials
// because the exec-based credential plugins (e.g. aws-okta-processor) may
// trigger interactive prompts (application selection, MFA) that bypass the
// automated flags configured by kubectl-eks. When the cached credentials
// turn out to be expired, the next kubectl command will fail and the user
// can re-run with --reauth.
func FindContextForCluster(clusterARN string) (string, bool) {
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	config, err := loadingRules.Load()
	if err != nil {
		return "", false
	}

	for name, ctx := range config.Contexts {
		if ctx.Cluster == clusterARN {
			return name, true
		}
	}
	return "", false
}

// UseContext switches the current kubeconfig context.
func UseContext(contextName string) error {
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	config, err := loadingRules.Load()
	if err != nil {
		return err
	}

	if _, exists := config.Contexts[contextName]; !exists {
		return fmt.Errorf("context %q not found", contextName)
	}

	config.CurrentContext = contextName
	return clientcmd.WriteToFile(*config, loadingRules.GetDefaultFilename())
}
