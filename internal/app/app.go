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

type config struct {
	dockerfile string
	context    string
	buildx     bool
	platform   string
	cacheFrom  string
	cacheTo    string
}

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

	if config.buildx {
		buildWithBuildx(config, image, runner)
	} else {
		build(config, image, runner)
	}

	return image, nil
}

func build(config *config, image string, runner CommandRunner) {
	fmt.Fprintf(os.Stderr, "$ docker build -f %s -t %s %s\n\n", config.dockerfile, image, config.context)
	runner.Run("docker", "build", "-f", config.dockerfile, "-t", image, config.context)

	fmt.Fprintf(os.Stderr, "\n- Pushing docker image...\n\n")
	fmt.Fprintf(os.Stderr, "$ docker push %s\n\n", image)
	runner.Run("docker", "push", image)
}

func buildWithBuildx(config *config, image string, runner CommandRunner) {
	qemuInstallArgs := []string{"run", "--privileged", "--rm", "tonistiigi/binfmt", "--install", "all"}

	fmt.Fprintf(os.Stderr, "$ docker %s\n\n", strings.Join(qemuInstallArgs, " "))
	runner.Run("docker", qemuInstallArgs...)

	builderCreateArgs := []string{"buildx", "create", "--bootstrap", "--use", "--name", "container", "--driver", "docker-container"}

	fmt.Fprintf(os.Stderr, "$ docker %s\n\n", strings.Join(builderCreateArgs, " "))
	runner.Run("docker", builderCreateArgs...)

	buildArgs := []string{"buildx", "build", "--push"}
	if config.platform != "" {
		buildArgs = append(buildArgs, "--platform", config.platform)
	}

	if config.cacheFrom != "" {
		buildArgs = append(buildArgs, "--cache-from", config.cacheFrom)
	}

	if config.cacheTo != "" {
		buildArgs = append(buildArgs, "--cache-to", config.cacheTo)
	}

	buildArgs = append(buildArgs, "-f", config.dockerfile, "-t", image, config.context)

	fmt.Fprintf(os.Stderr, "$ docker %s\n\n", strings.Join(buildArgs, " "))
	runner.Run("docker", buildArgs...)
}

func getConfig(buildID string, params map[string]interface{}) (*config, error) {
	result := config{
		dockerfile: "Dockerfile",
		context:    ".",
	}

	dockerFileI, ok := params["dockerfile"]
	if ok {
		result.dockerfile, ok = dockerFileI.(string)
		if !ok {
			return nil, fmt.Errorf("unexpected type for build.%v.params.dockerfile: %T (should be string)", buildID, dockerFileI)
		}
	}

	contextI, ok := params["context"]
	if ok {
		result.context, ok = contextI.(string)
		if !ok {
			return nil, fmt.Errorf("unexpected type for build.%v.params.context: %T (should be string)", buildID, contextI)
		}
	}

	buildxI, ok := params["buildx"]
	if ok {
		result.buildx, ok = buildxI.(bool)
		if !ok {
			return nil, fmt.Errorf("unexpected type for build.%v.params.buildx: %T (should be bool)", buildID, buildxI)
		}
	}

	platformI, ok := params["platform"]
	if ok {
		result.platform, ok = platformI.(string)
		if !ok {
			return nil, fmt.Errorf("unexpected type for build.%v.params.platform: %T (should be string)", buildID, platformI)
		}
	}

	cacheFromI, ok := params["cache-from"]
	if ok {
		result.cacheFrom, ok = cacheFromI.(string)
		if !ok {
			return nil, fmt.Errorf("unexpected type for build.%v.params.cache-from: %T (should be string)", buildID, cacheFromI)
		}
	}

	cacheToI, ok := params["cache-to"]
	if ok {
		result.cacheTo, ok = cacheToI.(string)
		if !ok {
			return nil, fmt.Errorf("unexpected type for build.%v.params.cache-to: %T (should be string)", buildID, cacheToI)
		}
	}

	if result.cacheFrom != "" {
		if !strings.Contains(result.cacheFrom, "type=gha") {
			return nil, fmt.Errorf("currently only gha cache type supported, got: %s", result.cacheFrom)
		}
	}

	if result.cacheTo != "" {
		if !strings.Contains(result.cacheTo, "type=gha") {
			return nil, fmt.Errorf("currently only gha cache type supported, got: %s", result.cacheTo)
		}
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
