FROM python:3.13-slim@sha256:d98cc29d3266e535dd9da1099688d9cb3ca4753813571e071763d17725460962
WORKDIR /opt/headroom
COPY requirements.txt ./
RUN pip install --no-cache-dir --require-hashes -r requirements.txt \
    && useradd --uid 10001 --create-home headroom
USER headroom
# Only deploy on the private compose network; never publish this port.
ENV HEADROOM_ALLOW_UNAUTHENTICATED_BIND=1 \
    HEADROOM_BEACON=off \
    HEADROOM_TELEMETRY=off \
    DO_NOT_TRACK=1
EXPOSE 8787
HEALTHCHECK --interval=30s --timeout=5s --start-period=60s CMD python -c "import urllib.request; urllib.request.urlopen('http://127.0.0.1:8787/health', timeout=4)"
ENTRYPOINT ["headroom", "proxy"]
CMD ["--host", "0.0.0.0", "--port", "8787", "--disable-kompress"]
