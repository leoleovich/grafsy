#
# Image for building packages in docker
# $IMAGE:builder on docker-hub
#
ARG IMAGE=leoleovich/grafsy
FROM golang:buster as builder

RUN apt-get update && apt-get install -y rpm ruby-dev libacl1-dev && \
    gem install --no-user-install --no-document fpm && \
    apt-get clean
