FROM golang:1-buster as builder

WORKDIR /go/src/kubectl-dev
COPY go.mod go.sum ./
RUN go mod download

COPY cmd/f2cm/main.go ./
RUN CGO_ENABLED=0 go build -o f2cm .

FROM scratch
WORKDIR /
COPY --from=builder /go/src/kubectl-dev/f2cm ./
ENTRYPOINT ["/f2cm"]
