FROM golang:1.14-alpine3.12 AS builder
ARG PKG_NAME=github.com/catorpilor/ethChef

WORKDIR /go/src/${PKG_NAME}
COPY . .
WORKDIR cmd
RUN CGO_ENABLED=0 go build -mod vendor -o /chef

FROM cheshire42/alpine:3.9
COPY --from=builder /chef /chef


EXPOSE 8090
ENTRYPOINT ["/chef"]