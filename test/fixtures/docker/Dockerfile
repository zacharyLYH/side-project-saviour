FROM alpine:3.20
# Fixture image for the Docker integration tests: python3 serves a test
# HTTP server, iproute2 provides ss for port detection. Node joins when
# harness phases (8+) need it.
RUN apk add --no-cache python3 iproute2
