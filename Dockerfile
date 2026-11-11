# Build a static binary, run it as non-root on distroless.
FROM golang:1.27 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /inventory ./cmd/inventory

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /inventory /inventory
EXPOSE 8080
USER 65532:65532
ENTRYPOINT ["/inventory"]
