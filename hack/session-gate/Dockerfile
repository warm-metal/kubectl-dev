FROM golang:1-buster as builder

WORKDIR /go/src/kubectl-dev
COPY go.mod go.sum ./
RUN go mod download

COPY cmd ./cmd
COPY pkg ./pkg
RUN CGO_ENABLED=0 go build \
    -o session-gate \
    ./cmd/session-gate

FROM scratch
COPY --from=builder /go/src/kubectl-dev/session-gate .
CMD ["./session-gate"]
