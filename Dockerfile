FROM golang:alpine AS builder

RUN apk add --update --no-cache git
RUN apk update
RUN apk upgrade

ADD . /go/src/github.com/veekeefr/cachet-monitor

RUN set -ex && \
    cd /go/src/github.com/veekeefr/cachet-monitor && \
    CGO_ENABLED=0 && \
    go get . && \
    go build -ldflags "-X main.AppBranch=${CIRCLE_BRANCH} -X main.Build=${CIRCLE_SHA1} -X main.BuildDate=`date +%Y-%m-%d_%H:%M:%S`" -o cachet_monitor && \
    mv ./cachet_monitor /usr/bin/cachet_monitor

FROM golang:alpine

COPY --from=builder /usr/bin/cachet_monitor /usr/local/bin/cachet_monitor

RUN mkdir -p /www/log
RUN mkdir -p /www/conf

VOLUME /www/log
VOLUME /www/conf

ENTRYPOINT [ "cachet_monitor", "-c", "/www/conf/cachet-monitor.yml, "--log=/www/log/cachet-monitor.log" ]
