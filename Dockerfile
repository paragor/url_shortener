FROM alpine:3.20.2

WORKDIR /app

COPY url_shortener /usr/bin/
ENTRYPOINT ["/usr/bin/url_shortener"]
