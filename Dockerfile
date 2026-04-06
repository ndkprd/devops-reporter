FROM golang:1.26-alpine AS builder

WORKDIR /build

COPY go.mod .
COPY cmd/ ./cmd/

RUN go build -o /argocd-report ./cmd/

FROM alpine:3.23

COPY --from=builder /argocd-report /usr/local/bin/argocd-report

ENTRYPOINT ["argocd-report"]
