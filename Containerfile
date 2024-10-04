FROM debian:bookworm-slim
RUN apt update && apt install -y sqlite3 imagemagick exiftool && apt-get clean
WORKDIR /app/data
COPY belverde-fire /app/belverde-fire
ENTRYPOINT ["/app/belverde-fire"]
