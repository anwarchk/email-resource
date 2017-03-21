FROM ubuntu:latest
ADD bin /opt/resource
RUN apt-get update && apt-get install -y ca-certificates
