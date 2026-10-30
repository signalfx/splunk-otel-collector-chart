FROM node:22.23.4-slim

RUN apt-get update && apt-get install -y --no-install-recommends bash curl && rm -rf /var/lib/apt/lists/*

COPY index.js start.sh /

ENTRYPOINT ["bash", "start.sh"]
