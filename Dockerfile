FROM alpine:3.5
EXPOSE 9102 9125/udp

LABEL container.name="wehkamp/statsd-exporter"

ENV  GOPATH /go
ENV APPPATH $GOPATH/src/github.com/docker-infra/statsd-exporter

ADD *.go $APPPATH/
ADD vendor $APPPATH/

RUN apk add --update -t build-deps go git libc-dev gcc libgcc && \
	cd $APPPATH && \
	go get -d && go build -o /bin/statsd-exporter && \
	apk del --purge build-deps && \
	rm -rf $GOPATH

ENTRYPOINT  [ "/bin/statsd-exporter" ]
