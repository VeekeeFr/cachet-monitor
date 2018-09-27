FROM golang:alpine AS builder

RUN apk add --update --no-cache git
RUN apk update
RUN apk upgrade

ADD . /go/src/github.com/veekeefr/cachet-monitor

RUN set -ex && \
    cd /go/src/github.com/veekeefr/cachet-monitor && \
    chmod +x scripts/*.sh && \
    CGO_ENABLED=0 && \
    go get . && \
    go build -ldflags "-X main.AppBranch=${CIRCLE_BRANCH} -X main.Build=${CIRCLE_SHA1} -X main.BuildDate=`date +%Y-%m-%d_%H:%M:%S`" -o cachet_monitor && \
    mv ./cachet_monitor /usr/bin/cachet_monitor

FROM centos:7

COPY --from=builder /usr/bin/cachet_monitor /usr/local/bin/cachet_monitor
COPY --from=builder /go/src/github.com/veekeefr/cachet-monitor/scripts/startCachet.sh /usr/local/bin/startCachet.sh

RUN chmod +x /usr/local/bin/cachet_monitor
RUN chmod +x /usr/local/bin/startCachet.sh

RUN mkdir -p /www/log
RUN mkdir -p /www/conf
RUN mkdir -p /www/scripts

VOLUME /www/log
VOLUME /www/conf
VOLUME /www/scripts

ENTRYPOINT [ "/usr/local/bin/startCachet.sh" ]
