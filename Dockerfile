FROM golang:1.18 as builder

WORKDIR /workspace

# Copy the Go Modules manifests
COPY go.mod go.mod
COPY go.sum go.sum

ENV GOPROXY "https://goproxy.cn,direct"

RUN go mod download

# Copy the go source
COPY main.go main.go
COPY controller/ controller/

# Build
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -o shell_exporter main.go


FROM debian:stable

WORKDIR /

RUN apt-get update && apt-get -y --no-install-recommends install curl net-tools procps mariadb-client  cron &&  apt-get autoremove -y && apt-get clean && rm -rf /var/lib/apt/lists/*

ENV DIR /exporters

ENV PORT ":9099"

COPY --from=builder /workspace/shell_exporter .

COPY entrypoint.sh /entrypoint.sh

COPY tini /tini

RUN chmod +x /shell_exporter && chmod +x entrypoint.sh && chmod +x /tini && mkdir /exporters

RUN echo "SHELL=/bin/bash\nMAILTO=root\n*/1 * * * * /cron.sh" | crontab -

EXPOSE 9099

ENTRYPOINT ["/tini", "--"]

CMD ["/entrypoint.sh"]

