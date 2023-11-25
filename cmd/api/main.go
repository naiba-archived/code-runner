package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"os"
	"os/signal"
	"time"

	"github.com/docker/distribution/uuid"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/client"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/basicauth"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/samber/lo"

	"github.com/naiba/code-runner/internal/apiio"
	"github.com/naiba/code-runner/internal/global"
	"github.com/naiba/code-runner/internal/model"
)

var conf *model.Config
var cli *client.Client

const execTimeout = time.Second * 60

func init() {
	var err error
	cli, err = client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		panic(err)
	}
	var innerConf model.Config
	data, err := os.ReadFile("data/config.json")
	if err != nil {
		panic(err)
	}
	err = json.Unmarshal(data, &innerConf)
	if err != nil {
		panic(err)
	}
	conf = &innerConf

	ctx := context.Background()
	// refresh docker images
	for _, img := range model.Runners {
		cli.ImagePull(ctx, img.Image, types.ImagePullOptions{})
	}
}

func main() {
	app := fiber.New(fiber.Config{
		ReadTimeout: time.Second * 5, // optimize for graceful shutdown
	})
	app.Use(logger.New())
	app.Use(recover.New())
	app.Get("/", func(c *fiber.Ctx) error {
		return c.SendString("Hello, World 👋!")
	})

	api := app.Group("api")
	{
		api.Use(basicauth.New(basicauth.Config{
			Users: conf.Clients,
		}))
		api.Get("/", func(c *fiber.Ctx) error {
			return c.SendString("It works!")
		})
		api.Get("/list", handleRunner)
		api.Post("/run", handleRunCode)
	}

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	go func() {
		<-c
		log.Println("Shutdown Server ...")
		global.CancelGlobal()
		app.Shutdown()
	}()

	app.Listen(":3000")
}

func handleRunner(c *fiber.Ctx) error {
	return c.JSON(model.Runners)
}

func handleRunCode(c *fiber.Ctx) error {
	var task apiio.RunCodeRequest
	if err := c.BodyParser(&task); err != nil {
		return err
	}

	dockerImage, has := model.Runners[task.Container]
	if !has {
		return errors.New("image not found")
	}

	fileID := uuid.Generate().String()

	path, err := os.Getwd()
	if err != nil {
		return err
	}
	path += "/"
	localFilename := path + "data/temp/" + fileID
	if err := os.WriteFile(localFilename, []byte(task.Code), os.FileMode(0777)); err != nil {
		return err
	}
	defer os.Remove(localFilename)

	fileName := conf.Temp + fileID

	var limit container.Resources
	if conf.Limit {
		limit = container.Resources{
			NanoCPUs: dockerImage.Limit.CPU * 10000000,
			Memory:   dockerImage.Limit.Mem * 1024 * 1024,
		}
	}

	resp, err := cli.ContainerCreate(global.Context, &container.Config{
		Image: dockerImage.Image,
		Cmd:   dockerImage.CMD,
		Tty:   false,
	}, &container.HostConfig{
		NetworkMode: "none",
		Mounts: []mount.Mount{
			{
				Type:   mount.TypeBind,
				Source: fileName,
				Target: dockerImage.Target,
			},
		},
		Resources: limit,
	}, nil, nil, "")
	if err != nil {
		return err
	}

	log.Println("exec", fileName, "in")
	defer func() {
		log.Println("exec", fileName, "exit")
		timout := 0
		cli.ContainerStop(global.Context, resp.ID, container.StopOptions{
			Timeout: &timout,
		})
		cli.ContainerRemove(global.Context, resp.ID, types.ContainerRemoveOptions{
			RemoveVolumes: true,
			Force:         true,
		})
	}()

	if err := cli.ContainerStart(global.Context, resp.ID, types.ContainerStartOptions{}); err != nil {
		return err
	}

	exitBody, outLog, err, ok := lo.TryOr3(func() (*container.WaitResponse, []byte, error, error) { return runCode(resp.ID) }, nil, nil, nil)
	if !ok {
		return err
	}

	var status int
	if exitBody.StatusCode == 0 {
		status = 1
	}

	return c.JSON(map[string]interface{}{
		"code": status,
		"out":  string(outLog),
	})
}

func runCode(containerId string) (*container.WaitResponse, []byte, error, error) {
	var err error
	var exitBody container.WaitResponse
	var outLog []byte

	timeout := time.NewTimer(execTimeout)
	waitResponse, errChan := cli.ContainerWait(global.Context, containerId, container.WaitConditionNotRunning)

	select {
	case errC := <-errChan:
		timeout.Stop()
		err = errC
	case resp := <-waitResponse:
		timeout.Stop()
		exitBody = resp
	case <-timeout.C:
		err = errors.New("execute timeout")
	}

	if err != nil {
		return &exitBody, outLog, err, err
	}

	out, err := cli.ContainerLogs(global.Context, containerId, types.ContainerLogsOptions{
		ShowStdout: true,
		ShowStderr: true,
	})
	if err != nil {
		return &exitBody, outLog, err, err
	}

	outLog, err = io.ReadAll(out)
	return &exitBody, outLog, err, err
}
