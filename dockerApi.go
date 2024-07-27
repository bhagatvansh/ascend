package main

import (
	"context"
	"io"
	"log"
	"os"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/archive"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/docker/go-connections/nat"
)

func GetContext(filePath string) io.Reader {
	ctx, _ := archive.TarWithOptions(filePath, &archive.TarOptions{})
	return ctx
}

func DockerAPI(deployRequest DeployRequest) {
	log.Print("Hello Docker")
	ctx := context.Background()
	dockerClient, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		log.Fatal(err)
	}
	defer dockerClient.Close()

	buildImage(dockerClient, deployRequest)

	startContainer(dockerClient, ctx, deployRequest)
}

func buildImage(dockerClient *client.Client, deployRequest DeployRequest) {
	dockerBuildContext := GetContext("./Dockerfile")
	// docker build --build-arg GIT_URL=https://github.com/theankitbhardwaj/latest-wayback-snapshot-redis.git --build-arg BUILD_CMD="go build -tags netgo -ldflags '-s -w' -o myService" --build-arg START_CMD="./myService" -t go-webservice .
	buildArgs := make(map[string]*string)

	buildArgs["GIT_URL"] = &deployRequest.Git_URL
	buildArgs["PORT"] = &deployRequest.Port
	// TODO: Send custom build and start commands

	buildOptions := types.ImageBuildOptions{
		Tags:      []string{"go-ascend"}, // TODO: Randomly generate this tag, this will be the subdomain for the user
		Remove:    true,
		BuildArgs: buildArgs,
		NoCache:   true,
	}
	buildResponse, err := dockerClient.ImageBuild(context.Background(), dockerBuildContext, buildOptions)
	if err != nil {
		log.Fatal(err)
	}
	io.Copy(os.Stdout, buildResponse.Body)

	defer buildResponse.Body.Close()
}

func startContainer(dockerClient *client.Client, ctx context.Context, deployRequest DeployRequest) {
	portBindings := make(nat.PortMap)
	containerPort := deployRequest.Port + "/tcp" // TODO: Better way to do this
	bindings := []nat.PortBinding{
		{HostIP: "", HostPort: "5562"}, //TODO: Get this host port dynamically
	}
	portBindings[nat.Port(containerPort)] = bindings

	resp, err := dockerClient.ContainerCreate(ctx, &container.Config{
		Image: "go-ascend",
	}, &container.HostConfig{
		PortBindings: portBindings,
		NetworkMode:  "bridge",
	}, nil, nil, "")

	if err != nil {
		log.Fatal(err)
	}

	if err := dockerClient.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
		log.Fatal(err)
	}

	statusCh, errCh := dockerClient.ContainerWait(ctx, resp.ID, container.WaitConditionNotRunning)
	select {
	case err := <-errCh:
		if err != nil {
			log.Fatal(err)
		}
	case <-statusCh:
	}

	out, err1 := dockerClient.ContainerLogs(ctx, resp.ID, container.LogsOptions{ShowStdout: true})
	if err1 != nil {
		log.Fatal(err1)
	}

	stdcopy.StdCopy(os.Stdout, os.Stderr, out)
}