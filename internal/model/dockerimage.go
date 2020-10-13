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

var resourceLimit = &ResourceLimit{
	CPU: 50,
	Mem: 50,
}

func init() {
	Runners = map[string]DockerImage{
		"golang-latest": {
			Image:  "golang:alpine",
			Target: "/runner/main.go",
			Template: `package main

			func main() {
				print("Hello world!\n")
			}
			`,
			CMD:   []string{"sh", "-c", "set -x && cd /runner && go build -o main main.go && ./main"},
			Limit: resourceLimit,
		},
		"gcc-latest": {
			Image:  "frolvlad/alpine-gcc:latest",
			Target: "/runner/main.c",
			Template: `int main()
			{ 
				printf("Hell%d w%drld!\n",0,0);
			}`,
			CMD:   []string{"sh", "-c", "set -x && cd /runner && gcc -o main main.c && ./main"},
			Limit: resourceLimit,
		},
	}
}
