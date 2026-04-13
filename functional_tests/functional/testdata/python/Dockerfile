FROM python:3.13-slim

RUN apt-get update && apt-get install -y --no-install-recommends bash curl && rm -rf /var/lib/apt/lists/*

RUN pip install --no-cache-dir Flask

COPY start.sh /
COPY web.py /

ENTRYPOINT ["bash", "start.sh"]
