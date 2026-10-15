package eks

import (
	"fmt"
	"os"
	"os/exec"
)

var runAWSCommand = func(cmd *exec.Cmd) error { return cmd.Run() }

func UpdateKubeConfig(profile, region, clusterName, kubeConfig string) error {
	var cmd *exec.Cmd

	// using the shell, run aws eks update-kubeconfig --name <clusterName> --region <region> --profile <profile>
	if kubeConfig != "" {
		cmd = exec.Command("aws", "eks", "update-kubeconfig", "--name", clusterName, "--region", region, "--profile", profile, "--kubeconfig", kubeConfig)
	} else {
		cmd = exec.Command("aws", "eks", "update-kubeconfig", "--name", clusterName, "--region", region, "--profile", profile)
	}

	cmd.Stdout = nil
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func buildEnhancedCredentialProcess(credentialProcess, factor, application string, noAWSCache bool) string {
	enhanced := credentialProcess
	if factor != "" {
		enhanced += " --factor=" + factor
	}
	if application != "" {
		enhanced += " --application=" + application
	}
	if noAWSCache {
		enhanced += " --no-aws-cache"
	}
	return enhanced
}

func buildTempAWSConfig(profile, credentialProcess string) string {
	return fmt.Sprintf("[profile %s]\ncredential_process=%s\n", profile, credentialProcess)
}

func buildUpdateKubeconfigArgs(profile, region, clusterName, kubeConfig string) []string {
	args := []string{"eks", "update-kubeconfig", "--name", clusterName, "--region", region, "--profile", profile}
	if kubeConfig != "" {
		args = append(args, "--kubeconfig", kubeConfig)
	}
	return args
}

func UpdateKubeConfigWithEnhancedCredentials(kubeconfigProfile, credentialProcess, factor, application, region, clusterName, kubeConfig string, noAWSCache bool) error {
	enhanced := buildEnhancedCredentialProcess(credentialProcess, factor, application, noAWSCache)

	tmpFile, err := os.CreateTemp("", "kubectl-eks-awsconfig-*.ini")
	if err != nil {
		return fmt.Errorf("failed to create temp AWS config: %w", err)
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)

	if _, err := tmpFile.WriteString(buildTempAWSConfig(kubeconfigProfile, enhanced)); err != nil {
		tmpFile.Close()
		return fmt.Errorf("failed to write temp AWS config: %w", err)
	}
	tmpFile.Close()

	cmd := exec.Command("aws", buildUpdateKubeconfigArgs(kubeconfigProfile, region, clusterName, kubeConfig)...)
	cmd.Env = append(os.Environ(), "AWS_CONFIG_FILE="+tmpPath)
	cmd.Stdin = os.Stdin
	cmd.Stderr = os.Stderr
	return runAWSCommand(cmd)
}
