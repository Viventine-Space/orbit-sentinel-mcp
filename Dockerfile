FROM golang:1.25-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /orbit-sentinel-mcp .

FROM alpine:3.21
RUN apk add --no-cache ca-certificates
COPY --from=build /orbit-sentinel-mcp /usr/local/bin/orbit-sentinel-mcp
ENTRYPOINT ["/usr/local/bin/orbit-sentinel-mcp"]
