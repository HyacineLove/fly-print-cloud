ARG DOCKERHUB_MIRROR=docker.1ms.run
FROM ${DOCKERHUB_MIRROR}/library/golang:1.25-alpine AS build
WORKDIR /src
COPY go.mod ./
COPY main.go ./
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/integration-demo .

FROM ${DOCKERHUB_MIRROR}/library/alpine:3.22
RUN addgroup -S demo && adduser -S -G demo demo
WORKDIR /app
COPY --from=build /out/integration-demo /app/integration-demo
RUN mkdir -p /data/files && chown -R demo:demo /data
USER demo
EXPOSE 8080
ENTRYPOINT ["/app/integration-demo"]
