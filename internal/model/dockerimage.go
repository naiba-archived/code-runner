package model

type ResourceLimit struct {
	CPU int64 `json:"cpu,omitempty"` // 0.0001 * CPU of cpu
	Mem int64 `json:"mem,omitempty"` // mb
}

type DockerImage struct {
	Image    string         `json:"image,omitempty"`
	Target   string         `json:"target,omitempty"`
	Template string         `json:"template,omitempty"`
	CMD      []string       `json:"cmd,omitempty"`
	Limit    *ResourceLimit `json:"limit,omitempty"`
}

var Runners map[string]DockerImage

func init() {
	Runners = map[string]DockerImage{
		"golang-latest": {
			Image:    "golang:alpine",
			Target:   "/runner/main.go",
			Template: `package main\nfunc main(){\nprint("hello world!")}`,
			CMD:      []string{"sh", "-c", "set -x && cd /runner && go build -o main main.go && ./main"},
			Limit: &ResourceLimit{
				CPU: 10,
				Mem: 50,
			},
		},
	}
}
