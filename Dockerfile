FROM ubuntu:18.04
RUN  apt-get update
RUN apt-get install -y  libpcap-dev
COPY habridge  /habridge
RUN chmod +x /habridge
ENTRYPOINT ./habridge