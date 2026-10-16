FROM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd ./cmd
COPY internal ./internal
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/radarr-mcp ./cmd/radarr-mcp

FROM gcr.io/distroless/static-debian13:nonroot
COPY --from=build /out/radarr-mcp /radarr-mcp
USER nonroot:nonroot
EXPOSE 8080
HEALTHCHECK --interval=30s --timeout=3s --retries=3 CMD ["/radarr-mcp", "--healthcheck"]
ENTRYPOINT ["/radarr-mcp"]
