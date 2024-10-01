FROM debian:bookworm-slim
WORKDIR /app/data
COPY belverde-fire /app/belverde-fire
ENTRYPOINT ["/app/belverde-fire"]
