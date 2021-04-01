#FROM ubuntu:18.04
#RUN  apt-get update
#RUN apt-get install -y  libpcap-dev
FROM 10.100.100.200/library/libpcap-base:latest
COPY habridge  /habridge
RUN chmod +x /habridge
ENTRYPOINT ./habridge
