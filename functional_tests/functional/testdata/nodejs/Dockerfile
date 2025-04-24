FROM node:18-alpine

RUN apk add --no-cache bash curl

COPY index.js launch.sh request-loop.sh /

ENTRYPOINT ["bash", "launch.sh"]
