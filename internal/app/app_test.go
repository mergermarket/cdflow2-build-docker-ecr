package app_test

import (
	"encoding/base64"
	"log"
	"reflect"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ecr"
	"github.com/aws/aws-sdk-go/service/ecr/ecriface"

	"github.com/mergermarket/cdflow-release-docker-ecr/internal/app"
)

type mockECRClient struct {
	ecriface.ECRAPI
}

func (mock *mockECRClient) GetAuthorizationToken(input *ecr.GetAuthorizationTokenInput) (*ecr.GetAuthorizationTokenOutput, error) {
	return &ecr.GetAuthorizationTokenOutput{
		AuthorizationData: []*ecr.AuthorizationData{
			{AuthorizationToken: aws.String(base64.StdEncoding.EncodeToString([]byte("test-username:test-password")))},
		},
	}, nil
}

type called struct {
	command string
	args    []string
	input   string
}

type mockCommandRunner struct {
	app.CommandRunner
	commands []called
}

func (mock *mockCommandRunner) Run(command string, args ...string) {
	mock.commands = append(mock.commands, called{command: command, args: args})
}

func (mock *mockCommandRunner) RunWithInput(input, command string, args ...string) {
	mock.commands = append(mock.commands, called{command: command, args: args, input: input})
}
func (mock *mockCommandRunner) RunWithOutput(command string, args ...string) (string, error) {
	mock.commands = append(mock.commands, called{command: command, args: args})
	return "[[driver-type io.containerd.snapshotter.v1]]", nil // Mocking output as empty for simplicity
}

func TestRun(t *testing.T) {
	// Given
	ecr := &mockECRClient{}
	commandRunner := &mockCommandRunner{}

	params := map[string]interface{}{}

	// When
	app.Run(ecr, commandRunner, params, "test-repository", "test-build-id", "test-version")

	// Then
	if !reflect.DeepEqual(commandRunner.commands, []called{
		{command: "docker", args: []string{"login", "-u", "test-username", "--password-stdin", "test-repository"}, input: "test-password"},
		{command: "docker", args: []string{"build", "-f", "Dockerfile", "-t", "test-repository:test-build-id-test-version", "."}},
		{command: "docker", args: []string{"push", "test-repository:test-build-id-test-version"}},
	}) {
		log.Fatal("unexpected commands:", commandRunner.commands)
	}

}

func TestRunWithParams(t *testing.T) {
	// Given
	ecr := &mockECRClient{}
	commandRunner := &mockCommandRunner{}

	params := map[string]interface{}{
		"dockerfile": "test1.Dockerfile",
		"context":    "./test",
	}

	// When
	app.Run(ecr, commandRunner, params, "test-repository", "test-build-id", "test-version")

	// Then
	if !reflect.DeepEqual(commandRunner.commands, []called{
		{command: "docker", args: []string{"login", "-u", "test-username", "--password-stdin", "test-repository"}, input: "test-password"},
		{command: "docker", args: []string{"build", "-f", "test1.Dockerfile", "-t", "test-repository:test-build-id-test-version", "./test"}},
		{command: "docker", args: []string{"push", "test-repository:test-build-id-test-version"}},
	}) {
		log.Fatal("unexpected commands:", commandRunner.commands)
	}
}

func TestBuildxRun(t *testing.T) {
	// Given
	ecr := &mockECRClient{}
	commandRunner := &mockCommandRunner{}

	params := map[string]interface{}{
		"buildx":    true,
		"platforms": "linux/arm64,linux/386,linux/s390x",
	}

	// When
	app.Run(ecr, commandRunner, params, "test-repository", "test-build-id", "test-version")

	// Then
	if !reflect.DeepEqual(commandRunner.commands, []called{
		{command: "docker", args: []string{"login", "-u", "test-username", "--password-stdin", "test-repository"}, input: "test-password"},
		{command: "docker", args: []string{"run", "--privileged", "--rm", "tonistiigi/binfmt", "--install", "all"}},
		{command: "docker", args: []string{"buildx", "create", "--bootstrap", "--use", "--name", "container", "--driver", "docker-container", "--config", "/etc/buildkit/buildkitd.toml"}},
		{command: "docker", args: []string{"info", "-f", "{{.DriverStatus}}"}},
		{command: "docker", args: []string{"buildx", "build", "--push", "--load", "--platform", "linux/arm64,linux/386,linux/s390x", "-f", "Dockerfile", "-t", "test-repository:test-build-id-test-version", "."}},
	}) {
		log.Fatal("unexpected commands:", commandRunner.commands)
	}
}
