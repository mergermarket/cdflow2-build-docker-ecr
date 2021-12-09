package app

import (
	"bufio"
	"encoding/base64"
	"fmt"
	"log"
	"os"
	"regexp"
	"strings"

	"github.com/aws/aws-sdk-go/service/ecr"
	"github.com/aws/aws-sdk-go/service/ecr/ecriface"
	"github.com/docker/docker/api"
	"github.com/docker/docker/registry"
)

// Run runs the build process.
func Run(ecrClient ecriface.ECRAPI, runner CommandRunner, params map[string]interface{}, repository, buildID, version string) (string, error) {
	config, err := getConfig(buildID, params)
	if err != nil {
		return "", fmt.Errorf("error getting config: %w", err)
	}

	fmt.Fprintf(os.Stderr, "- Getting ECR auth token...\n")
	username, password := getCredentials(ecrClient)

	fmt.Fprintf(os.Stderr, "- Authenticating docker client to ECR repository...\n\n")
	fmt.Fprintf(os.Stderr, "$ docker login -u %s --password-stdin %s\n\n", username, repository)
	runner.RunWithInput(password, "docker", "login", "-u", username, "--password-stdin", repository)

	image := repository + ":" + buildID + "-" + version

	attemptToLoginToRegistriesInDockerFile(runner)
	fmt.Fprintf(os.Stderr, "\n- Building docker image...\n\n")
	fmt.Fprintf(os.Stderr, "$ docker build -t %s .\n\n", image)
	runner.Run("docker", "build", "-f", config.dockerfile, "-t", image, ".")

	fmt.Fprintf(os.Stderr, "\n- Pushing docker image...\n\n")
	fmt.Fprintf(os.Stderr, "$ docker push %s\n\n", image)
	runner.Run("docker", "push", image)

	return image, nil
}

type config struct {
	dockerfile string
}

func getConfig(buildID string, params map[string]interface{}) (*config, error) {
	result := config{}
	ok := false
	if _, exists := params["dockerfile"]; !exists {
		result.dockerfile = "Dockerfile"
	} else if result.dockerfile, ok = params["dockerfile"].(string); !ok {
		return nil, fmt.Errorf("unexpected type for build.%v.params.dockerfile: %T (should be string)", buildID, params["dockerfile"])
	}
	return &result, nil
}

func getCredentials(ecrClient ecriface.ECRAPI) (string, string) {
	result, err := ecrClient.GetAuthorizationToken(&ecr.GetAuthorizationTokenInput{})
	if err != nil {
		log.Fatal(err)
	}
	auth := result.AuthorizationData[0]
	data, err := base64.StdEncoding.DecodeString(*auth.AuthorizationToken)
	if err != nil {
		log.Fatal(err)
	}
	credentials := strings.Split(string(data), ":")
	return credentials[0], credentials[1]
}

func attemptToLoginToRegistriesInDockerFile(runner CommandRunner) {
	dockerfileFromLinePattern := regexp.MustCompile(`(?i)^[\s]*FROM[ \f\r\t\v]+(?P<image>[^ \f\r\t\v\n#]+)`)
	cwd, err := os.Getwd()
	if err != nil {
		log.Fatal(err)
	}
	dockerfile, err := os.Open(cwd + "/Dockerfile")
	if err != nil {
		log.Print(err)
		return
	}
	defer dockerfile.Close()
	scanner := bufio.NewScanner(dockerfile)
	for scanner.Scan() {
		line := scanner.Text()

		matches := dockerfileFromLinePattern.FindStringSubmatch(line)
		if matches != nil && matches[1] != api.NoBaseImageSpecifier {
			imageRegistry := registry.IndexHostname
			if strings.Count(matches[1], "/") > 1 {
				imageRegistry = strings.Split(matches[1], "/")[0]
			}
			registryVarName := strings.ToUpper(
				strings.NewReplacer(
					".", "_",
					":", "_",
					"-", "_",
				).Replace(imageRegistry),
			)

			cdflowDockerAuthPrefix := "CDFLOW2_DOCKER_AUTH_"
			username := os.Getenv(cdflowDockerAuthPrefix + registryVarName + "_USERNAME")
			password := os.Getenv(cdflowDockerAuthPrefix + registryVarName + "_PASSWORD")

			if len(username) > 0 && len(password) > 0 {
				fmt.Fprintf(os.Stderr, "- Found credentials for registry %s. Attempting to login...\n\n", imageRegistry)
				runner.RunWithInput(password, "docker", "login", "-u", username, "--password-stdin", imageRegistry)
			} else {
				fmt.Fprintf(os.Stderr, "- Auth credentials not found for registry '%s'.\n", imageRegistry)
				fmt.Fprintf(os.Stderr, "Access to this registry will be without auth.\n")
				fmt.Fprintf(os.Stderr, "Set the appropriate environment variables if auth is required.\n\n")
			}
		}
	}
}
