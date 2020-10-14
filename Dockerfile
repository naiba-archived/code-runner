FROM golang:alpine AS binarybuilder
RUN apk --no-cache --no-progress add \
    gcc \
    musl-dev
WORKDIR /coderunner
COPY . .
RUN cd cmd/api \
    && go build -o api -ldflags="-s -w"
FROM alpine:latest
RUN apk --no-cache --no-progress add \
    ca-certificates \
    tzdata
WORKDIR /coderunner
COPY dist /coderunner/dist
COPY --from=binarybuilder /coderunner/cmd/api/api ./api

VOLUME ["/coderunner/data"]
EXPOSE 8080
CMD ["/coderunner/api"]