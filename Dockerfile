FROM gcr.io/distroless/base
COPY flux-helm-version-updater /
ENTRYPOINT ["/flux-helm-version-updater"]
