FROM archlinux:latest
RUN pacman -Syu --noconfirm sqlite3 imagemagick perl-image-exiftool libheif
ENV PATH="$PATH:/usr/bin/vendor_perl"
WORKDIR /app/data
COPY belverde-fire /app/belverde-fire
ENTRYPOINT ["/app/belverde-fire"]
