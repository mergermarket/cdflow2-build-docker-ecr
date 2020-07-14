package app

import (
	"encoding/base64"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/aws/aws-sdk-go/service/ecr"
	"github.com/aws/aws-sdk-go/service/ecr/ecriface"
)

// Run runs the build process.
func Run(ecrClient ecriface.ECRAPI, runner CommandRunner, repository, buildID, version string) string {
	fmt.Fprintf(os.Stderr, "- Getting ECR auth token...\n")
	username, password := getCredentials(ecrClient)

	fmt.Fprintf(os.Stderr, "- Authenticating docker client to ECR repository...\n\n")
	fmt.Fprintf(os.Stderr, "$ docker login -u %s --password-stdin %s\n\n", username, repository)
	runner.RunWithInput(password, "docker", "login", "-u", username, "--password-stdin", repository)

	image := repository + ":" + buildID + "-" + version

	fmt.Fprintf(os.Stderr, "\n- Building docker image...\n\n")
	fmt.Fprintf(os.Stderr, "$ docker build -t %s .\n\n", image)
	runner.Run("docker", "build", "-t", image, ".")

	fmt.Fprintf(os.Stderr, "\n- Pushing docker image...\n\n")
	fmt.Fprintf(os.Stderr, "$ docker push %s\n\n", image)
	runner.Run("docker", "push", image)

	return image
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
