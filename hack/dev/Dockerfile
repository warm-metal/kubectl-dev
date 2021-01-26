FROM golang:1-buster as builder

WORKDIR /go/src/kubectl-dev
COPY go.mod go.sum ./
RUN go mod download
COPY .git ./.git

FROM builder as linux
ENV GOOS=linux
COPY cmd ./cmd
COPY pkg ./pkg
RUN CGO_ENABLED=0 go build \
    -o kubectl-dev \
    -ldflags "-X github.com/warm-metal/kubectl-dev/pkg/release.Version=$(git for-each-ref --points-at=HEAD --format='%(refname:short)' refs/tags --sort=-version:refname --count=2)$(git symbolic-ref -q --short HEAD) -X github.com/warm-metal/kubectl-dev/pkg/release.Commit=$(git rev-parse HEAD)" \
    ./cmd/dev

FROM builder as macos
ENV GOOS=darwin
COPY cmd ./cmd
COPY pkg ./pkg
RUN CGO_ENABLED=0 go build \
    -o kubectl-dev \
    -ldflags "-X github.com/warm-metal/kubectl-dev/pkg/release.Version=$(git for-each-ref --points-at=HEAD --format='%(refname:short)' refs/tags --sort=-version:refname --count=2)$(git symbolic-ref -q --short HEAD) -X github.com/warm-metal/kubectl-dev/pkg/release.Commit=$(git rev-parse HEAD)" \
    ./cmd/dev

FROM bash:5 as app
COPY --from=linux /go/src/kubectl-dev/kubectl-dev /usr/local/bin/
ENTRYPOINT ["tail", "-f", "/dev/null"]

FROM scratch as linux-cli
COPY --from=linux /go/src/kubectl-dev/kubectl-dev .

FROM scratch as mac-cli
COPY --from=macos /go/src/kubectl-dev/kubectl-dev .