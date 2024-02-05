# udptlspipe changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog][keepachangelog], and this project
adheres to [Semantic Versioning][semver].

[keepachangelog]: https://keepachangelog.com/en/1.0.0/

[semver]: https://semver.org/spec/v2.0.0.html

## [Unreleased]

[unreleased]: https://github.com/ameshkov/udptlspipe/compare/v1.2.0...HEAD

## [1.2.0] - 2024-02-05

* Changed the protocol for messages exchange, it now uses WebSocket internally.
  This change allows running `udptlspipe` server behind a CDN if that CDN
  supports WebSocket.

[1.2.0]: https://github.com/ameshkov/udptlspipe/releases/tag/v1.2.0

## [1.1.0] - 2024-02-03

* Added an option to configure custom TLS certificate. Check out
  [README][readmetlscert] for more information on how to use that.

[1.1.0]: https://github.com/ameshkov/udptlspipe/releases/tag/v1.1.0

[readmetlscert]: https://github.com/ameshkov/udptlspipe?tab=readme-ov-file#tlscert

## [1.0.1] - 2024-02-02

* Added a [docker image][dockerregistry].

[dockerregistry]: https://github.com/ameshkov/udptlspipe/pkgs/container/udptlspipe

[1.0.1]: https://github.com/ameshkov/udptlspipe/releases/tag/v1.0.1

## [1.0.0] - 2024-02-02

### Added

* The first version with base functionality.

[1.0.0]: https://github.com/ameshkov/udptlspipe/releases/tag/v1.0.0