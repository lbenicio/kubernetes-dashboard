# Changelog

All notable changes to this project are documented in this file. The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and uses [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Changed

- Updated the development container's `kubectl` to Kubernetes v1.37.2. The local Kind test cluster remains on the latest node image published by Kind v0.33.0 (Kubernetes v1.37.0).
- Configured local development and production services to accept IPv4 and IPv6 loopback traffic so `localhost` works with either address family.

## [7.14.10] - 2026-11-05

### Added

- Added unit coverage for API endpoint listing.
- Re-enabled end-to-end tests.
- Published API, Auth, Web, and Metrics Scraper container images with `7.14.10` and `latest` tags to Docker Hub and GHCR under `lbenicio/kubernetes-dashboard-*`.
- Added Linux amd64, arm/v7, arm64, ppc64le, and s390x image variants.

### Changed

- Upgraded Go to 1.27.3, Kubernetes libraries to v0.37.2, and Kubernetes development tooling to v1.37.2.
- Upgraded Angular through v21, migrated Angular APIs, and moved production builds to the Angular application builder.
- Updated the web toolchain to Node.js 24 and Yarn 4.18.1 with Corepack. Modernized Material theme loading, ESLint configuration, Jest's ESM setup, and frontend formatting.
- Refreshed web dependencies and tooling, including GraphQL 17.0.2, Zone.js 0.16.3, tslib 2.9.1, js-yaml 5.5.0, Prettier 3.9.10, concurrently 10.0.6, del 9.0.1, semver 7.8.5, wait-on 9.5.1, and run-node 3.0.0.
- Replaced legacy flex-layout and browser packages with CSS or native browser APIs; updated terminal packages and bundled the Ace editor script.
- Updated Docker release and CI workflows to use `DOCKER_USER` and `DOCKER_PAT` for Docker Hub and publish to both registries. Refreshed GitHub Actions for checkout, Go and Node setup, Docker build and publish, Buildx, QEMU, Codecov, dependency review, and kind-action.
- Updated Dockerfiles to run Go and user-file build stages on BuildKit's build platform and cross-compile Go binaries for target architectures.
- Updated Kong gateway route protocols, isolated local Kind kubeconfigs, and installed architecture-matched development CLI tools.
- Updated repository badges and README guidance, license checks, Docker build contexts, and generated-file exclusions.

### Fixed

- Corrected namespace selector sizing, alignment, menu width, and close/reset behavior.
- Refined table action placement, row spacing, detail-page layout, and spacing around empty and populated lists.
- Reworked logs, help content, resource properties, and status chart layouts; restored spacing below hidden detail rows.
- Updated API and test code for current Kubernetes discovery interfaces and fixed bundled editor loading.

### Security

- Patched vulnerable frontend dependencies and updated deepmerge handling in CI.
- Prevented caching of user and CSRF responses.
- Restricted end-to-end test namespace permissions.

### Removed

- Removed obsolete frontend dependencies, including SystemJS, legacy polyfills, `file-saver`, and `crop-url`.
- Removed the legacy Helm chart and Helm CI/CD workflows, the pull-request workflow, and outdated community governance files.
- Removed the archived-project notice from the README.
