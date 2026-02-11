#
# Image which contains the binary artefacts
#
FROM golang:trixie AS build


COPY . ./grafsy

WORKDIR ./grafsy

RUN apt update && \
    apt install -y libacl1-dev && \
    make clean test && \
    make build

#
# Application image
#
FROM debian:stable-slim

RUN apt-get update && apt-get install libacl1 -y && apt-get clean && mkdir /etc/grafsy

WORKDIR /grafsy

COPY --from=build /go/grafsy/build/grafsy ./grafsy

COPY --from=build /go/grafsy/build/grafsy-client ./grafsy-client

COPY entrypoint.sh /entrypoint.sh

ENTRYPOINT ["/entrypoint.sh"]
CMD ["/grafsy/grafsy"]
