FROM node:16-alpine

RUN apk add --no-cache bash curl

COPY index.js start.sh /

ENTRYPOINT ["bash", "start.sh"]
