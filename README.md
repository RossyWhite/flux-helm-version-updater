[![build](https://github.com/RossyWhite/flux-helm-version-updater/actions/workflows/release.yaml/badge.svg)](https://github.com/RossyWhite/flux-helm-version-updater/actions/workflows/release.yaml)
[![Release](https://img.shields.io/github/release/RossyWhite/flux-helm-version-updater.svg)](https://github.com/RossyWhite/flux-helm-version-updater/releases/latest)

# flux-helm-version-updater

`flux-helm-version-updater` enables update automation of [Fluxv2 Helm Releases](https://fluxcd.io/docs/components/helm/helmreleases/).

It works as follows,

1. Fetch All `HelmReleases` resources in the running cluster
2. Get the latest chart version from their underlying Helm Repository
3. If it is newer than current one, create Pull Request to rewrite the version tag

## How to use

1. Add a marker to the target HelmRelease yaml
    - in `{"$helmversionupdate": "namespace:helmreleasename"}` format
2. Run flux-helm-version-updater command

You can find an example [here](./examples).
