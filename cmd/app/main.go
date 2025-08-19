package main

import (
	"encoding/json"
	"log"
	"os"

	"github.com/mergermarket/cdflow-release-docker-ecr/internal/app"

	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ecr"
)

func main() {
	if len(os.Args) == 2 && os.Args[1] == "requirements" {
		// requirements is a way for the release container to communciate its requirements to the
		// config container
		if err := json.NewEncoder(os.Stdout).Encode(map[string]interface{}{
			"needs": []string{"ecr", "gha"},
		}); err != nil {
			log.Panicln("error encoding requirements:", err)
		}
		return
	}
	// declared above
	repository := os.Getenv("ECR_REPOSITORY")

	// built-in
	buildID := os.Getenv("BUILD_ID")
	version := os.Getenv("VERSION")

	params := map[string]interface{}{}
	if err := json.Unmarshal([]byte(os.Getenv("MANIFEST_PARAMS")), &params); err != nil {
		log.Fatalln("error loading MANIFEST_PARAMS:", err)
	}

	result, err := app.Run(
		ecr.New(session.Must(session.NewSession())),
		&app.ExecCommandRunner{OutputStream: os.Stdout, ErrorStream: os.Stderr},
		params,
		repository,
		buildID,
		version,
	)
	if err != nil {
		log.Fatal(err)
	}

	if err := os.WriteFile("/release-metadata.json", []byte(result), 0644); err != nil {
		log.Fatalln("error writing release metadata:", err)
	}
}
