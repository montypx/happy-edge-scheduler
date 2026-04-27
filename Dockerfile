FROM golang:1.26.2 AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o bin/kube-scheduler ./cmd/scheduler

FROM gcr.io/distroless/static:nonroot
COPY --from=builder /app/bin/kube-scheduler /usr/local/bin/kube-scheduler
USER nonroot:nonroot
ENTRYPOINT ["/usr/local/bin/kube-scheduler"]