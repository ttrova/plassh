# Build stage
FROM golang:1.25-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /bin/plassh ./cmd/server

# Final stage
FROM alpine:3.20
RUN adduser -D -u 10001 app
WORKDIR /data
COPY --from=build /bin/plassh /usr/local/bin/plassh
USER app
EXPOSE 2222
ENV SSH_HOST_KEY=/data/host_key
ENTRYPOINT ["plassh"]
