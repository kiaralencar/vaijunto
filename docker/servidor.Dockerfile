FROM golang:1.23-alpine AS build
WORKDIR /src
COPY go.mod ./
COPY internal ./internal
COPY cmd ./cmd
RUN CGO_ENABLED=0 go build -o /out/servidor ./cmd/servidor

FROM alpine:3.20
COPY --from=build /out/servidor /servidor
EXPOSE 8080
ENTRYPOINT ["/servidor", "8080"]
