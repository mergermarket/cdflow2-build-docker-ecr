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
	called int
}

func (mock *mockECRClient) GetAuthorizationToken(input *ecr.GetAuthorizationTokenInput) (*ecr.GetAuthorizationTokenOutput, error) {
	mock.called++
	return &ecr.GetAuthorizationTokenOutput{
		AuthorizationData: []*ecr.AuthorizationData{
			&ecr.AuthorizationData{AuthorizationToken: aws.String(base64.StdEncoding.EncodeToString([]byte("test-username:test-password")))},
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

func TestRun(t *testing.T) {
	// Given
	ecr := &mockECRClient{}
	commandRunner := &mockCommandRunner{}

	// When
	app.Run(ecr, commandRunner, "test-registry", "test-build-id", "test-version")

	// Then
	if !reflect.DeepEqual(commandRunner.commands, []called{
		{command: "docker", args: []string{"login", "-u", "test-username", "--password-stdin", "test-registry"}, input: "test-password"},
		{command: "docker", args: []string{"build", "-t", "test-registry:test-build-id-test-version", "."}},
		{command: "docker", args: []string{"push", "test-registry:test-build-id-test-version"}},
	}) {
		log.Fatal("unexpected commands:", commandRunner.commands)
	}

}
