FROM golang:1.25-bookworm AS build

WORKDIR /src
COPY go.mod ./
COPY go.sum ./
COPY cmd ./cmd
COPY internal ./internal
RUN go test ./... && CGO_ENABLED=1 go build -o /out/sav_cli ./cmd/sav_cli

FROM debian:bookworm-slim
RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates libstdc++6 \
    && rm -rf /var/lib/apt/lists/*
COPY --from=build /out/sav_cli /sav_cli
ENV SAVE_INDEXER_HOST=0.0.0.0
ENV SAVE_INDEXER_PORT=8090
EXPOSE 8090
ENTRYPOINT ["/sav_cli"]
CMD ["serve"]
