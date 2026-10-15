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
		}
	}

	return &awsConfig
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

func GetOktaEnhancements(profileName string) (credentialProcess, factor, app string) {
	if ConfigData == nil {
		loadAWSConfig()
	}

	p, ok := ConfigData.Profiles[profileName]
	if !ok {
		return "", "", ""
	}
	credentialProcess = p.CredentialProcess

	factor = p.OktaFactor
	if factor == "" {
		factor = ConfigData.OktaFactor
	}

	app = p.AWSApp
	if app == "" {
		app = ConfigData.AWSApp
	}

	return credentialProcess, factor, app
}

func GetRolePreference(profileName string) []string {
	if ConfigData == nil {
		loadAWSConfig()
	}

	if p, ok := ConfigData.Profiles[profileName]; ok && len(p.RolePreference) > 0 {
		return p.RolePreference
	}
	return ConfigData.RolePreference
}
