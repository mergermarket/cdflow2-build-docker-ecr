# mergermarket/cdflow-build-docker-ecr

* [mergermarket/cdflow-build-docker-ecr on DockerHub](https://hub.docker.com/r/mergermarket/cdflow2-build-docker-ecr).
* [mergermarket/cdflow-build-docker-ecr on GitHub](https://github.com/mergermarket/cdflow2-build-docker-ecr).

[cdflow2](https://developer-preview.acuris.com/opensource/cdflow2/) build plugin for building docker images and pushing them to [AWS ECR](https://aws.amazon.com/ecr/). Performs the following steps:

* Gets an auth token for ECR based on IAM credentials in the environment.
* Does a `docker login` with that auth token in order to allow a docker image to be pushed to the repo.
* Does a `docker build` to create a docker image from the `Dockerfile` in the root of the project.
* Does a `docker push` to push the image to the ECR repository.
* Provides an `image` release metadata key so the resulting docker image can be used from terraform - via a terraform map variable named the same as the build (i.e. the key under `builds` in the `cdflow.yaml` - "docker" is a good choice)

Requires a cdflow2 config container with support for providing ECR config - e.g. [mergermarket/cdflow2-config-acuris](https://hub.docker.com/r/mergermarket/cdflow2-config-acuris).

## Usage

### cdflow.yaml

This example uses `mergermarket/cdflow2-config-acuris`, which supports creating an ECR repository and providing the config for it as environment variables to the build (this config container is only suitable for developing within Acuris).

```yaml
version: 2
config:
  image: mergermarket/cdflow2-config-acuris
  params:
    account_prefix: myaccountprefix
    team: myteam
builds:
  docker:
    image: mergermarket/cdflow2-build-docker-ecr
    params:
      dockerfile: Dockerfile
      context: .
terraform:
  image: hashicorp/terraform
```

### Dockerfile

Will build a docker image from a Dockerfile in the root of the project. This could be anything, in this case a simple hello world.

```Dockerfile
FROM hello-world
```

### Parameters

#### dockerfile

Change the Dockerfile path used for the build.  
Default path is `Dockerfile` in the same directory as the `cdflow.yaml`.

#### context

Change the context of the build. A build’s context is the set of files located in the specified path.  
Defaults to `.`.

#### buildx

Use buildx for image creation with multi architecture support instead of the classic build command.  
Boolean parameter.  
Defaults to `false`.

#### platform

Comma-separated list of platforms for the build, when buildx is enabled.  
E.g.: `linux/arm64,linux/386,linux/s390x`.  
If buildx not enabled, parameter ignored.  
Defaults to empty list.

## Config container support

Config containers that supports this build plugin are:

* [mergermarket/cdflow2-config-acuris](https://hub.docker.com/r/mergermarket/cdflow2-config-acuris).
* [mergermarket/cdflow2-config-aws-simple](https://github.com/mergermarket/cdflow2-config-aws-simple)

### Adding support in a config container

This container advertises a single `"ecr"` need when it is configured for a build. A config container in its `configureRelease` hook should ensure that an ECR repository exists and is provided in the environment, along with AWS credentials that can push to it and a region:

* `ECR_REPOSITORY` - the address of the repository of the form `<account-number>.dkr.ecr.<region>.amazonaws.com/<repo-name>`)
* `AWS_ACCESS_KEY_ID`
* `AWS_SECRET_ACCESS_KEY`
* `AWS_SESSION_TOKEN` (for temporary credentials only)
* `AWS_REGION`
