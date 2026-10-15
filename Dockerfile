FROM gcr.io/distroless/static-debian13:nonroot
ARG TARGETPLATFORM
COPY $TARGETPLATFORM/zammadbridge /usr/local/bin/zammadbridge
# Mount the configuration at /etc/3cx-zammad-bridge/config.yaml
ENTRYPOINT ["/usr/local/bin/zammadbridge"]
