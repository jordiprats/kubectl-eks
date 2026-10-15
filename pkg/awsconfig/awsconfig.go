package awsconfig

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"os"
	"strings"

	"github.com/jordiprats/kubectl-eks/pkg/data"
)

var ConfigData *data.AWSConfig = nil

func loadAWSConfigFromReader(reader io.Reader) *data.AWSConfig {
	awsConfig := data.AWSConfig{Profiles: make(map[string]data.AWSProfile)}
	awsConfig.Profiles[""] = data.AWSProfile{}

	scanner := bufio.NewScanner(reader)
	currentProfile := ""
	for scanner.Scan() {
		line := scanner.Text()

		// remove leading and trailing spaces
		line = strings.TrimSpace(line)

		if strings.HasPrefix(line, "# kubectl-eks-regions=") {
			parts := strings.Split(line, "=")
			if len(parts) != 2 {
				continue
			}
			eksRegions := strings.Split(parts[1], ",")

			for i, region := range eksRegions {
				eksRegions[i] = strings.TrimSpace(region)
			}

			currentProfileDetails := awsConfig.Profiles[currentProfile]
			currentProfileDetails.HintEKSRegions = eksRegions
			awsConfig.Profiles[currentProfile] = currentProfileDetails
		}

		if strings.HasPrefix(line, "# kubectl-eks-kubeconfig-profile=") {
			value := strings.TrimPrefix(line, "# kubectl-eks-kubeconfig-profile=")
			value = strings.TrimSpace(value)
			if currentProfile == "" {
				awsConfig.KubeconfigProfile = value
			} else {
				p := awsConfig.Profiles[currentProfile]
				p.KubeconfigProfile = value
				awsConfig.Profiles[currentProfile] = p
			}
		}

		if strings.HasPrefix(line, "# kubectl-eks-okta-factor=") {
			value := strings.TrimPrefix(line, "# kubectl-eks-okta-factor=")
			value = strings.TrimSpace(value)
			if currentProfile == "" {
				awsConfig.OktaFactor = value
			} else {
				p := awsConfig.Profiles[currentProfile]
				p.OktaFactor = value
				awsConfig.Profiles[currentProfile] = p
			}
		}

		if strings.HasPrefix(line, "# kubectl-eks-okta-app-url=") {
			value := strings.TrimPrefix(line, "# kubectl-eks-okta-app-url=")
			value = strings.TrimSpace(value)
			if currentProfile == "" {
				awsConfig.AWSApp = value
			} else {
				p := awsConfig.Profiles[currentProfile]
				p.AWSApp = value
				awsConfig.Profiles[currentProfile] = p
			}
		}

		if strings.HasPrefix(line, "# kubectl-eks-role-preference=") {
			value := strings.TrimPrefix(line, "# kubectl-eks-role-preference=")
			parts := strings.Split(value, ",")
			for i, p := range parts {
				parts[i] = strings.TrimSpace(p)
			}
			if currentProfile == "" {
				awsConfig.RolePreference = parts
			} else {
				p := awsConfig.Profiles[currentProfile]
				p.RolePreference = parts
				awsConfig.Profiles[currentProfile] = p
			}
		}

		// get rid of comments
		if strings.Contains(line, "#") {
			line = strings.Split(line, "#")[0]
		}

		// remove leading and trailing spaces
		line = strings.TrimSpace(line)

		// skip empty lines
		if len(line) == 0 {
			continue
		}

		// [profile example]
		if strings.HasPrefix(line, "[profile ") {
			currentProfile = strings.TrimSuffix(strings.TrimPrefix(line, "[profile "), "]")
			awsConfig.Profiles[currentProfile] = data.AWSProfile{Name: currentProfile}
		}

		parts := strings.SplitN(line, "=", 2)

		// skip lines that are not key=value
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		switch key {
		case "region":
			currentProfileDetails := awsConfig.Profiles[currentProfile]
			currentProfileDetails.DefaultRegion = value
			awsConfig.Profiles[currentProfile] = currentProfileDetails
		case "credential_process":
			currentProfileDetails := awsConfig.Profiles[currentProfile]
			currentProfileDetails.CredentialProcess = value
			awsConfig.Profiles[currentProfile] = currentProfileDetails
		case "role_arn":
			currentProfileDetails := awsConfig.Profiles[currentProfile]
			currentProfileDetails.RoleARN = value
			awsConfig.Profiles[currentProfile] = currentProfileDetails
		case "sso_role_name":
			currentProfileDetails := awsConfig.Profiles[currentProfile]
			currentProfileDetails.SSORoleName = value
			awsConfig.Profiles[currentProfile] = currentProfileDetails
		}
	}

	return &awsConfig
}

// tryLoadAWSConfig loads ~/.aws/config if it exists, silently leaving
// ConfigData nil when the file is missing or unreadable.
func tryLoadAWSConfig() {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return
	}
	file, err := os.Open(homeDir + "/.aws/config")
	if err != nil {
		return
	}
	defer file.Close()
	ConfigData = loadAWSConfigFromReader(file)
}

func loadAWSConfig() {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		fmt.Println("Error retrieving UserHomeDir")
		os.Exit(1)
	}

	file, err := os.Open(homeDir + "/.aws/config")
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()

	ConfigData = loadAWSConfigFromReader(file)
}

func GetAWSProfilesWithEKSHints() []data.AWSProfile {
	if ConfigData == nil {
		loadAWSConfig()
	}

	// fmt.Printf("ConfigData: %+v\n", ConfigData)

	profiles := []data.AWSProfile{}
	for _, profileDetails := range ConfigData.Profiles {
		if len(profileDetails.HintEKSRegions) > 0 {
			profiles = append(profiles, profileDetails)
		}
	}

	return profiles
}

func GetEffectiveKubeconfigProfile(discoveryProfile string) string {
	if ConfigData == nil {
		loadAWSConfig()
	}

	if p, ok := ConfigData.Profiles[discoveryProfile]; ok && p.KubeconfigProfile != "" {
		return p.KubeconfigProfile
	}
	return ConfigData.KubeconfigProfile
}

func GetOktaEnhancements(discoveryProfile, kubeconfigProfile string) (credentialProcess, factor, app string) {
	if ConfigData == nil {
		loadAWSConfig()
	}

	kp, ok := ConfigData.Profiles[kubeconfigProfile]
	if !ok {
		return "", "", ""
	}
	dp := ConfigData.Profiles[discoveryProfile]

	factor = firstNonEmpty(dp.OktaFactor, kp.OktaFactor, ConfigData.OktaFactor)
	app = firstNonEmpty(dp.AWSApp, kp.AWSApp, ConfigData.AWSApp)

	return kp.CredentialProcess, factor, app
}

func GetRolePreference(discoveryProfile, kubeconfigProfile string) []string {
	if ConfigData == nil {
		loadAWSConfig()
	}

	if p := ConfigData.Profiles[discoveryProfile]; len(p.RolePreference) > 0 {
		return p.RolePreference
	}
	if p := ConfigData.Profiles[kubeconfigProfile]; len(p.RolePreference) > 0 {
		return p.RolePreference
	}
	return ConfigData.RolePreference
}

// GetProfileRoleName returns a short role name for the given AWS profile,
// extracted from ~/.aws/config without any AWS API calls.
// It uses role_arn (taking the name after ":role/") or sso_role_name.
// Returns "" if the profile has neither or the config can't be read.
func GetProfileRoleName(profile string) string {
	if profile == "" {
		return ""
	}
	if ConfigData == nil {
		tryLoadAWSConfig()
	}
	if ConfigData == nil {
		return ""
	}
	p, ok := ConfigData.Profiles[profile]
	if !ok {
		return ""
	}
	if p.RoleARN != "" {
		// arn:aws:iam::123456789012:role/path/RoleName -> path/RoleName
		if idx := strings.Index(p.RoleARN, ":role/"); idx >= 0 {
			return p.RoleARN[idx+len(":role/"):]
		}
		return p.RoleARN
	}
	return p.SSORoleName
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}
