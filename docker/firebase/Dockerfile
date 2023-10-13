FROM node:20

WORKDIR /opt

RUN set -eux; \
    apt update && \
    apt install -y openjdk-17-jre-headless && \
    apt clean && \
    npm install -g firebase-tools
