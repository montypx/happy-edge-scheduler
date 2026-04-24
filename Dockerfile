FROM gcr.io/distroless/static:nonroot
COPY bin/kube-scheduler /usr/local/bin/kube-scheduler
USER nonroot:nonroot
ENTRYPOINT ["/usr/local/bin/kube-scheduler"]