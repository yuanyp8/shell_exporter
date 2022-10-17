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


FROM docker.io/centos:7

RUN yum clean all && yum -y update && yum install -y net-tools iproute openssh-clients openssh-server crontabs which sudo
RUN groupadd -g 500 admin && useradd -g 500 -u 500 -d /home/admin -m admin
RUN echo 'admin ALL=(ALL) NOPASSWD:ALL' >> /etc/sudoers

WORKDIR /

ENV DIR /exporters
ENV PORT 9099
COPY --from=builder /workspace/shell_exporter .
COPY entrypoint.sh /entrypoint.sh

# Add Tini
ENV TINI_VERSION v0.19.0
ADD https://github.com/krallin/tini/releases/download/${TINI_VERSION}/tini /tini

RUN chmod +x /shell_exporter && chmod +x entrypoint.sh && chmod +x /tini

ENTRYPOINT ["/tini", "--"]

CMD ["/entrypoint.sh"]

