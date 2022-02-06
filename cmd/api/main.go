package main

import (
	"context"
	"encoding/json"
	"errors"
	"io/ioutil"
	"log"
	"os"
	"os/signal"
	"syscall"
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

	"github.com/naiba/code-runner/internal/apiio"
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
	data, err := ioutil.ReadFile("data/config.json")
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

	go app.Listen(":3000")

	// Wait for interrupt signal to gracefully shutdown the server with
	// a timeout of 5 seconds.
	quit := make(chan os.Signal, 1)
	// kill (no param) default send syscall.SIGTERM
	// kill -2 is syscall.SIGINT
	// kill -9 is syscall.SIGKILL but can't be catch, so don't need add it
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("Shutdown Server ...")

	if err := app.Shutdown(); err != nil {
		log.Fatal("Server Shutdown: ", err)
	}

	log.Println("Server exiting")
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
	if err := ioutil.WriteFile(localFilename, []byte(task.Code), os.FileMode(0777)); err != nil {
		return err
	}

	fileName := conf.Temp + fileID

	var limit container.Resources
	if conf.Limit {
		limit = container.Resources{
			NanoCPUs: dockerImage.Limit.CPU * 10000000,
			Memory:   dockerImage.Limit.Mem * 1024 * 1024,
		}
	}
	resp, err := cli.ContainerCreate(c.Context(), &container.Config{
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
		os.Remove(fileName)
		if err := cli.ContainerStop(c.Context(), resp.ID, nil); err != nil {
			panic(err)
		}
		if err := cli.ContainerRemove(c.Context(), resp.ID, types.ContainerRemoveOptions{
			RemoveVolumes: true,
			Force:         true,
		}); err != nil {
			panic(err)
		}
	}()

	if err := cli.ContainerStart(c.Context(), resp.ID, types.ContainerStartOptions{}); err != nil {
		return err
	}

	errChan := make(<-chan error)
	waitBody := make(<-chan container.ContainerWaitOKBody)
	timeout := time.NewTimer(execTimeout)

	go func() {
		waitBody, errChan = cli.ContainerWait(c.Context(), resp.ID, container.WaitConditionNotRunning)
	}()

	var errExec error
	var exitBody container.ContainerWaitOKBody
	select {
	case errC := <-errChan:
		timeout.Stop()
		errExec = errC
	case body := <-waitBody:
		timeout.Stop()
		exitBody = body
	case <-timeout.C:
		errExec = errors.New("execute timeout")
	}

	if errExec != nil {
		return errExec
	}

	out, err := cli.ContainerLogs(c.Context(), resp.ID, types.ContainerLogsOptions{
		ShowStdout: true,
		ShowStderr: true,
	})
	if err != nil {
		return err
	}

	outLog, err := ioutil.ReadAll(out)
	if err != nil {
		return err
	}

	var status int
	if exitBody.StatusCode == 0 {
		status = 1
	}

	c.JSON(map[string]interface{}{
		"code": status,
		"out":  string(outLog),
	})

	return nil
}
