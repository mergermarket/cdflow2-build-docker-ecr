package main

import (
	"encoding/json"
	"io/ioutil"
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
			"needs": []string{"ecr"},
		}); err != nil {
			log.Panicln("error  encoding requirements:", err)
		}
		return
	}
	// declared above
	repository := os.Getenv("ECR_REPOSITORY")

	// built-in
	buildID := os.Getenv("BUILD_ID")
	version := os.Getenv("VERSION")

	image := app.Run(
		ecr.New(session.Must(session.NewSession())),
		&app.ExecCommandRunner{OutputStream: os.Stdout, ErrorStream: os.Stderr},
		repository,
		buildID,
		version,
	)
	data, err := json.Marshal(map[string]string{"image": image})
	if err != nil {
		log.Fatal(err)
	}
	if err := ioutil.WriteFile("/release-metadata.json", data, 0644); err != nil {
		log.Fatalln("error writing release metadata:", err)
	}
}
