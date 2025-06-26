package app

import (
	"io"
	"log"
	"os"
	"os/exec"
	"strings"
)

// CommandRunner is something that can run commands.
type CommandRunner interface {
	Run(command string, args ...string)
	RunWithInput(input, command string, args ...string)
	RunWithOutput(command string, args ...string) (string, error)
}

// ExecCommandRunner implements the CommandRunner interface using exec.Command.
type ExecCommandRunner struct {
	OutputStream io.Writer
	ErrorStream  io.Writer
}

// Run runs a simple command.
func (runner *ExecCommandRunner) Run(command string, args ...string) {
	cmd := exec.Command(command, args...)
	cmd.Stdout = runner.OutputStream
	cmd.Stderr = runner.ErrorStream
	e := cmd.Run()
	if e != nil {
		os.Exit(1)
	}
}

// RunWithInput runs a simple command piping the supplied input to it.
func (runner *ExecCommandRunner) RunWithInput(input string, command string, args ...string) {
	cmd := exec.Command(command, args...)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		log.Fatal(err)
	}
	go func() {
		defer stdin.Close()
		io.WriteString(stdin, input)
	}()
	cmd.Stdout = runner.OutputStream
	cmd.Stderr = runner.ErrorStream
	if err := cmd.Run(); err != nil {
		log.Fatal(err)
		os.Exit(1)
	}
}

func (runner *ExecCommandRunner) RunWithOutput(command string, args ...string) (string, error) {
	outputBuffer := &strings.Builder{}
	cmd := exec.Command(command, args...)
	cmd.Stdout = outputBuffer
	cmd.Stderr = runner.ErrorStream
	if err := cmd.Run(); err != nil {
		return "", err
	}
	return outputBuffer.String(), nil
}
