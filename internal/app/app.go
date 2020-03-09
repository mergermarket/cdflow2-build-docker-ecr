package app

import (
	"encoding/base64"
	"log"
	"strings"

	"github.com/aws/aws-sdk-go/service/ecr"
	"github.com/aws/aws-sdk-go/service/ecr/ecriface"
)

// Run runs the build process.
func Run(ecrClient ecriface.ECRAPI, runner CommandRunner, registry, buildID, version string) string {

	username, password := getCredentials(ecrClient)
	runner.RunWithInput(password, "docker", "login", "-u", username, "--password-stdin", registry)

	image := registry + ":" + buildID + "-" + version

	runner.Run("docker", "build", "-t", image, ".")
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
