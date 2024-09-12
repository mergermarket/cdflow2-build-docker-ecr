package app

import (
	"bufio"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/aws/aws-sdk-go/service/ecr"
	"github.com/aws/aws-sdk-go/service/ecr/ecriface"
	"github.com/docker/distribution/reference"
	"github.com/docker/docker/api"
	"github.com/docker/docker/registry"
)

const (
	cdflowDockerAuthPrefix = "CDFLOW2_DOCKER_AUTH_"
)

type config struct {
	dockerfile string
	context    string
	buildx     bool
	platforms  string
	cacheFrom  string
	cacheTo    string
	secrets    []string
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
		err := buildWithBuildx(config, image, runner)
		if err != nil {
			return "", err
		}
	} else {
		build(config, image, runner)
	}

	data, err := json.Marshal(map[string]string{
		"image":     image,
		"buildx":    strconv.FormatBool(config.buildx),
		"platforms": config.platforms})
	if err != nil {
		log.Fatal(err)
	}

	return string(data), nil
}

func build(config *config, image string, runner CommandRunner) {
	buildArgs := []string{"build", "-f", config.dockerfile, "-t", image}
	for _, secret := range config.secrets {
		buildArgs = append(buildArgs, "--secret", secret)
	}
	buildArgs = append(buildArgs, config.context)
	fmt.Fprintf(os.Stderr, "$ docker %s\n\n", strings.Join(buildArgs, " "))
	runner.Run("docker", buildArgs...)

	fmt.Fprintf(os.Stderr, "\n- Pushing docker image...\n\n")
	fmt.Fprintf(os.Stderr, "$ docker push %s\n\n", image)
	runner.Run("docker", "push", image)
}

func buildWithBuildx(config *config, image string, runner CommandRunner) error {
	err := checkBuildxConfig(config)
	if err != nil {
		return err
	}

	qemuInstallArgs := []string{"run", "--privileged", "--rm", "tonistiigi/binfmt", "--install", "all"}

	fmt.Fprintf(os.Stderr, "$ docker %s\n\n", strings.Join(qemuInstallArgs, " "))
	runner.Run("docker", qemuInstallArgs...)

	builderCreateArgs := []string{"buildx", "create", "--bootstrap", "--use", "--name", "container", "--driver", "docker-container"}

	fmt.Fprintf(os.Stderr, "$ docker %s\n\n", strings.Join(builderCreateArgs, " "))
	runner.Run("docker", builderCreateArgs...)

	buildArgs := []string{"buildx", "build", "--push"}
	if config.platforms != "" {
		buildArgs = append(buildArgs, "--platform", config.platforms)
	}

	if config.cacheFrom != "" {
		buildArgs = append(buildArgs, "--cache-from", config.cacheFrom)
	}

	if config.cacheTo != "" {
		buildArgs = append(buildArgs, "--cache-to", config.cacheTo)
	}
	for _, secret := range config.secrets {
		buildArgs = append(buildArgs, "--secret", secret)
	}

	buildArgs = append(buildArgs, "-f", config.dockerfile, "-t", image, config.context)

	fmt.Fprintf(os.Stderr, "$ docker %s\n\n", strings.Join(buildArgs, " "))
	runner.Run("docker", buildArgs...)

	return nil
}

func checkBuildxConfig(config *config) error {
	if config.cacheFrom != "" {
		if !strings.Contains(config.cacheFrom, "type=gha") {
			return fmt.Errorf("currently only gha cache type supported, got: %s", config.cacheFrom)
		}
	}

	if config.cacheTo != "" {
		if !strings.Contains(config.cacheTo, "type=gha") {
			return fmt.Errorf("currently only gha cache type supported, got: %s", config.cacheTo)
		}
	}

	if config.cacheFrom != "" || config.cacheTo != "" {
		url := os.Getenv("ACTIONS_CACHE_URL")
		cache := os.Getenv("ACTIONS_RUNTIME_TOKEN")

		if url == "" || cache == "" {
			fmt.Fprintf(os.Stderr, "Github authentication parameter(s) missing, gha cache won't be used.\n\n")
		}
	}

	return nil
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

	platformsI, ok := params["platforms"]
	if ok {
		result.platforms, ok = platformsI.(string)
		if !ok {
			return nil, fmt.Errorf("unexpected type for build.%v.params.platforms: %T (should be string)", buildID, platformsI)
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
	secrets, ok := params["secrets"]
	if ok {
		for _, secret := range secrets.([]interface{}) {
			result.secrets = append(result.secrets, secret.(string))
		}
		if !ok {
			return nil, fmt.Errorf("unexpected type for build.%v.params.secrets: %T (should be []string)", buildID, secrets)
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
			username, password, imageRegistry := getRegistryAuth(matches[1])

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

func getRegistryAuth(image string) (username, password, imageRegistry string) {
	username, password, imageRegistry, err := getRegistryCredentials(image)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to get registry credentials, fallback to legacy method: %v\n\n", err)
	}

	if username == "" || password == "" {
		username, password, imageRegistry = getRegistryCredentialsLegacy(image)
	}

	return username, password, imageRegistry
}

func getRegistryCredentials(image string) (username, password, imageRegistry string, err error) {
	named, err := reference.ParseNormalizedNamed(image)
	if err != nil {
		return "", "", "", err
	}

	imageRegistry = reference.Domain(named)
	registryEnv := strings.ToUpper(
		strings.NewReplacer(
			".", "_",
			":", "_",
			"-", "_",
		).Replace(imageRegistry),
	)

	return os.Getenv(cdflowDockerAuthPrefix + registryEnv + "_USERNAME"), os.Getenv(cdflowDockerAuthPrefix + registryEnv + "_PASSWORD"), imageRegistry, nil
}

func getRegistryCredentialsLegacy(image string) (username, password, imageRegistry string) {
	imageRegistry = registry.IndexHostname
	if strings.Count(image, "/") > 1 {
		imageRegistry = strings.Split(image, "/")[0]
	}

	registryEnv := strings.ToUpper(
		strings.NewReplacer(
			".", "_",
			":", "_",
			"-", "_",
		).Replace(imageRegistry),
	)

	return os.Getenv(cdflowDockerAuthPrefix + registryEnv + "_USERNAME"), os.Getenv(cdflowDockerAuthPrefix + registryEnv + "_PASSWORD"), imageRegistry
}
