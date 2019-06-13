FROM alpine:latest

RUN apk update \
    && apk add --no-cache curl \
                          ca-certificates \
                          tzdata \
    && update-ca-certificates

RUN addgroup -S kube-operator && adduser -S -g kube-operator kube-operator
USER kube-operator

COPY alertmanager-config-controller /bin/alertmanager-config-controller

ENTRYPOINT ["/bin/alertmanager-config-controller"]
