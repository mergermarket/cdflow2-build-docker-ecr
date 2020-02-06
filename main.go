package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"strings"

	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ecr"
)

func main() {
	if len(os.Args) != 3 {
		fmt.Fprintln(os.Stderr, "component and version parameters required")
		os.Exit(1)
	}
	componentName := os.Args[1]
	version := os.Args[2]

	image := run(componentName, version)
	data, err := json.Marshal(map[string]string{"image": image})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(string(data))
}

func run(componentName string, version string) string {
	username, password, endpoint := getCredentials()
	dockerLogin(username, password, endpoint)

	registry := strings.Split(endpoint, "//")[1]
	image := fmt.Sprintf("%s/%s:%s", registry, componentName, version)

	runCommand("docker", "build", "-t", image, ".")
	runCommand("docker", "push", image)
	return image
}

func getCredentials() (string, string, string) {
	svc := ecr.New(session.New())
	input := &ecr.GetAuthorizationTokenInput{}
	result, err := svc.GetAuthorizationToken(input)
	if err != nil {
		log.Fatal(err)
	}
	auth := result.AuthorizationData[0]
	data, err := base64.StdEncoding.DecodeString(*auth.AuthorizationToken)
	if err != nil {
		log.Fatal(err)
	}
	credentials := strings.Split(string(data), ":")
	return credentials[0], credentials[1], *auth.ProxyEndpoint
}

func dockerLogin(username string, password string, endpoint string) {
	cmd := exec.Command("docker", "login", "-u", username, "--password-stdin", endpoint)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		log.Fatal(err)
	}
	go func() {
		defer stdin.Close()
		io.WriteString(stdin, password)
	}()
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		log.Fatal(err)
	}
}

func runCommand(command string, args ...string) {
	cmd := exec.Command(command, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	e := cmd.Run()
	if e != nil {
		os.Exit(1)
	}
}
