FROM golang:1.27.1-alpine3.24 AS build
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY cmd ./cmd
COPY internal ./internal
RUN CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o /server ./cmd/server
RUN CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o /create-admin ./cmd/create-admin

FROM alpine:3.24
RUN apk add --no-cache ca-certificates && addgroup -S app && adduser -S -G app app && mkdir -p /data/uploads && chown app:app /data/uploads
COPY --from=build /server /server
COPY --from=build /create-admin /create-admin
USER app
EXPOSE 8080
CMD ["/server"]
