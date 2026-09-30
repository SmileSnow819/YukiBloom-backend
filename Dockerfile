FROM golang:1.27.1-alpine3.24 AS build
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY cmd ./cmd
COPY internal ./internal
RUN CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o /server ./cmd/server
RUN CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o /create-admin ./cmd/create-admin
RUN CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o /import-posts ./cmd/import-posts
RUN CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o /import-personal ./cmd/import-personal
RUN CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o /import-site ./cmd/import-site

FROM alpine:3.24
RUN apk add --no-cache ca-certificates && addgroup -S app && adduser -S -G app app && mkdir -p /data/uploads && chown app:app /data/uploads
COPY --from=build /server /server
COPY --from=build /create-admin /create-admin
COPY --from=build /import-posts /import-posts
COPY --from=build /import-personal /import-personal
COPY --from=build /import-site /import-site
USER app
EXPOSE 8080
CMD ["/server"]
