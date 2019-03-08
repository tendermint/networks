FROM golang:alpine

ENV DEPS make git libc-dev bash gcc linux-headers eudev-dev curl

ENV BLOCKS 1000
ENV SEED 1
ENV PERIOD 50
ENV SLACK_URL ''
ENV GO111MODULE 'on'

WORKDIR /go/src/github.com/cosmos

RUN apk add --no-cache $DEPS && \
    git clone https://github.com/cosmos/cosmos-sdk.git && cd cosmos-sdk && \
    git checkout develop && \
    make go-mod-cache

WORKDIR /go/src/github.com/cosmos/cosmos-sdk

COPY multisim_ecs.sh multisim_ecs.sh
COPY notify_slack.go notify_slack.go

ENTRYPOINT ["/go/src/github.com/cosmos/cosmos-sdk/multisim_ecs.sh"]
