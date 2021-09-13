FROM docker:20.10.8-dind

RUN set -e && \
    ARCH=$(uname -m) && \
    URL=https://storage.googleapis.com/gvisor/releases/release/20210830/${ARCH} && \
    wget ${URL}/runsc ${URL}/runsc.sha512 && \
    sha512sum -c runsc.sha512 && \
    rm -f *.sha512 && \
    chmod a+rx runsc && \
    mv runsc /usr/local/bin && \
    runsc install

ENTRYPOINT ["dockerd-entrypoint.sh"]

