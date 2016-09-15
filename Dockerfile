# Create a minimal container to run a Golang static binary

FROM centurylink/ca-certs
LABEL repo="https://github.com/treeder/tiny-golang-docker"

WORKDIR /app

# copy binary into image
COPY wsh2s bricks.pac server.crt server.csr server.key ca.* /app/

ARG port=9080

EXPOSE ${port}

ENV GOMAXPROCS=1 PORT=${port}

ENTRYPOINT ["./wsh2s", "-log_dir=."]
