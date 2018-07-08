FROM        avezila/go

RUN groupadd go \
 && adduser -g go go \
 && mkdir -p /runtime \
 && chown -R go:go /runtime

USER go

WORKDIR /runtime

CMD  ["/buf"]

COPY         ./app /buf
