package app_test

import (
	"bytes"
	"log"
	"testing"

	"github.com/mergermarket/cdflow-release-docker-ecr/internal/app"
)

func TestRunnerRun(t *testing.T) {
	// Given
	var outputBuffer bytes.Buffer
	var errorBuffer bytes.Buffer
	runner := &app.ExecCommandRunner{OutputStream: &outputBuffer, ErrorStream: &errorBuffer}

	// When
	runner.Run("/bin/sh", "-c", "echo out; echo err >&2")

	// Then
	if outputBuffer.String() != "out\n" {
		log.Fatalln("unexpected output:", outputBuffer.String())
	}
	if errorBuffer.String() != "err\n" {
		log.Fatalln("unexpected errors:", errorBuffer.String())
	}
}

func TestRunnerRunWithInput(t *testing.T) {
	// Given
	var outputBuffer bytes.Buffer
	var errorBuffer bytes.Buffer
	runner := &app.ExecCommandRunner{OutputStream: &outputBuffer, ErrorStream: &errorBuffer}

	// When
	runner.RunWithInput("test-input", "/bin/sh", "-c", "input=$(cat); echo \"out: $input\"; echo \"err: $input\" >&2")

	// Then
	if outputBuffer.String() != "out: test-input\n" {
		log.Fatalln("unexpected output:", outputBuffer.String())
	}
	if errorBuffer.String() != "err: test-input\n" {
		log.Fatalln("unexpected errors:", errorBuffer.String())
	}
}

func TestRunnerRunWithOutput(t *testing.T) {
	// Given
	var outputBuffer bytes.Buffer
	var errorBuffer bytes.Buffer
	runner := &app.ExecCommandRunner{OutputStream: &outputBuffer, ErrorStream: &errorBuffer}

	// When
	output, _ := runner.RunWithOutput("/bin/sh", "-c", "echo out: test-output; echo err >&2")

	// Then
	if output != "out: test-output\n" {
		log.Fatalln("unexpected output:", output)
	}

}
