FROM golang:1.23-alpine AS build
WORKDIR /src
COPY go.mod ./
COPY internal ./internal
COPY cmd ./cmd
RUN CGO_ENABLED=0 go build -o /out/cliente-motorista ./cmd/cliente-motorista

FROM alpine:3.20
COPY --from=build /out/cliente-motorista /cliente-motorista
ENTRYPOINT ["/cliente-motorista"]
