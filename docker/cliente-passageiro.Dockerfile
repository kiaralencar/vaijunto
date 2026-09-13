FROM golang:1.23-alpine AS build
WORKDIR /src
COPY go.mod ./
COPY internal ./internal
COPY cmd ./cmd
RUN CGO_ENABLED=0 go build -o /out/cliente-passageiro ./cmd/cliente-passageiro

FROM alpine:3.20
COPY --from=build /out/cliente-passageiro /cliente-passageiro
ENTRYPOINT ["/cliente-passageiro"]
