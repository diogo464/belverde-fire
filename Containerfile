FROM alpine:latest
WORKDIR /app
COPY belverde-fire .
ENTRYPOINT ["/app/belverde-fire"]
