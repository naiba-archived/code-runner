package main

import (
	"encoding/json"
	"io/ioutil"
	"log"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/basicauth"
	"github.com/gofiber/fiber/v2/middleware/recover"

	"github.com/naiba/code-runner/internal/apiio"
	"github.com/naiba/code-runner/internal/model"
)

var conf *model.Config

func init() {
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
}

func main() {
	app := fiber.New()
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
		api.Post("/task", func(c *fiber.Ctx) error {
			var task apiio.Task
			if err := c.BodyParser(&task); err != nil {
				return err
			}
			log.Println(task)
			return nil
		})
	}

	app.Listen(":3000")
}
