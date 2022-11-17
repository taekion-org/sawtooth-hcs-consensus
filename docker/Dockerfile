FROM ubuntu:22.04

ARG TARGETARCH

RUN apt-get -y update
RUN apt-get -y install wget build-essential pkg-config libzmq3-dev
RUN wget https://go.dev/dl/go1.19.3.linux-${TARGETARCH}.tar.gz
RUN tar -C /usr/local -xzf go1.19.3.linux-${TARGETARCH}.tar.gz
ENV PATH $PATH:/usr/local/go/bin

COPY sawtooth-hcs-consensus /sawtooth-hcs-consensus
COPY sawtooth-sdk-go /sawtooth-sdk-go
RUN make -C sawtooth-hcs-consensus

FROM ubuntu:22.04
RUN apt-get -y update
RUN apt-get -y install libzmq3-dev
COPY --from=0 /sawtooth-hcs-consensus/build/sawtooth-hcs-consensus /sawtooth-hcs-consensus
ENV PATH $PATH:/
