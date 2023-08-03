FROM golang:1.20.7-bullseye

RUN apt-get -y update
RUN apt-get -y install wget build-essential pkg-config libzmq3-dev

COPY sawtooth-hcs-consensus /sawtooth-hcs-consensus
COPY sawtooth-sdk-go /sawtooth-sdk-go
RUN make -C /sawtooth-hcs-consensus

FROM debian:bullseye
RUN apt-get -y update
RUN apt-get -y install libzmq3-dev ca-certificates
COPY --from=0 /sawtooth-hcs-consensus/build/sawtooth-hcs-consensus /sawtooth-hcs-consensus
ENV PATH $PATH:/
