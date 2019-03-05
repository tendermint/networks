FROM golang:alpine

ENV DEPS make git libc-dev bash gcc linux-headers eudev-dev

ENV BLOCKS 1000
ENV SEED 1
ENV PERIOD 50

WORKDIR /go/src/github.com/cosmos

RUN apk add --no-cache $DEPS && \
    git clone https://github.com/cosmos/cosmos-sdk.git && cd cosmos-sdk && \
    git checkout develop && \
    make vendor-deps

COPY multisim_ecs.sh multisim_ecs.sh

ENTRYPOINT ["/go/src/github.com/cosmos/multisim_ecs.sh"]
