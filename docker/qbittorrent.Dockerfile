FROM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd ./cmd
COPY internal ./internal
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/qbittorrent-mcp ./cmd/qbittorrent-mcp

FROM gcr.io/distroless/static-debian13:nonroot
COPY --from=build /out/qbittorrent-mcp /qbittorrent-mcp
USER nonroot:nonroot
EXPOSE 8080
ENTRYPOINT ["/qbittorrent-mcp"]
