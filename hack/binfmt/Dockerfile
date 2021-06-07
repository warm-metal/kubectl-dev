FROM alpine:3.13
COPY --from=docker.io/tonistiigi/binfmt:qemu-v6.0.0 /usr/bin /usr/bin/
ENV HOST=/host/usr/bin
ENTRYPOINT "cp -pf /usr/bin/qemu-* ${HOST} && /usr/bin/binfmt --install all"