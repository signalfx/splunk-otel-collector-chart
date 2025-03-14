FROM python:3.13-alpine

RUN apk add --no-cache bash curl

RUN pip install Flask

COPY start.sh /
COPY web.py /

ENTRYPOINT ["bash", "start.sh"]
