# Remote Code Runner

:construction_worker: Docker-based remote code runner with simple API.

## Supported Languages

Go/GCC

## API

- List supported runners: `GET /api/runner`

    ```json
    {
    "golang-latest":{
        "image":"golang:alpine",
        "target":"/runner/main.go",
        "template":"package main\\nfunc main(){\\nprint(\"hello world!\")}",
        "cmd":[
            "sh",
            "-c",
            "set -x \u0026\u0026 cd /runner \u0026\u0026 go build -o main main.go \u0026\u0026 ./main"
        ],
        "limit":{
            "cpu":10,
            "mem":50
        }
    }
    }
    ```

- Run the code snippet: `POST /api/task`

    cURL:

    ```sh
    # dXNlcm5hbWU6cGFzc3dvcmQ=: base64(username:password)
    curl -X POST \
  'http://localhost:3000/api/task' \
  -H 'Authorization: Basic dXNlcm5hbWU6cGFzc3dvcmQ=' \
  -H 'Content-Type: application/json; charset=utf-8' \
  -d '{
   "content":"int main(){printf(\"Hello world!\");}",
   "container":"gcc-latest"'
    ```

    Request Body:

    ```json
    {
        "content":"package main\nfunc main() {print(\"hello world!\")}",
        "container":"golang-latest"
    }
    ```

    **(Don't forget the HTTP basic authentication header)**

    Response Body:

    ```json
    {
        "code": 0,
        "out": "\u0002\u0000\u0000\u0000\u0000\u0000\u0000\r+ cd /runner\n\u0002\u0000\u0000\u0000\u0000\u0000\u0000\u001b+ go build -o main main.go\n\u0002\u0000\u0000\u0000\u0000\u0000\u0000\u0019# command-line-arguments\n\u0002\u0000\u0000\u0000\u0000\u0000\u0000?./main.go:2:35: syntax error: unexpected x at end of statement\n"
    }
    ```