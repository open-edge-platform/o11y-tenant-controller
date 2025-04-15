<!--
SPDX-FileCopyrightText: (C) 2025 Intel Corporation
SPDX-License-Identifier: Apache-2.0
-->

# Edge Orchestrator Observability Tenant Controller

[alerting-monitor]: https://github.com/open-edge-platform/o11y-alerting-monitor
[edgenode-observability]: https://github.com/open-edge-platform/o11y-charts/tree/main/charts/edgenode-observability
[grafana-proxy]: https://github.com/open-edge-platform/o11y-charts/tree/main/apps/grafana-proxy
[orchestrator-observability]: https://github.com/open-edge-platform/o11y-charts/tree/main/charts/orchestrator-observability

[prometheus-agent]: https://github.com/open-edge-platform/edge-manageability-framework/blob/main/argocd/applications/templates/orchestrator-prometheus-agent.yaml
[sre-exporter]: https://github.com/open-edge-platform/o11y-sre-exporter
[tenancy-datamodel]: https://github.com/open-edge-platform/orch-utils/tree/main/tenancy-datamodel

[Documentation]: https://github.com/open-edge-platform/orch-docs
[Edge Orchestrator Community]: https://github.com/open-edge-platform
[Troubleshooting]: https://github.com/open-edge-platform/orch-docs
[Contact us]: https://github.com/open-edge-platform

[Apache 2.0 License]: LICENSES/Apache-2.0.txt
[Contributor's Guide]: https://docs.openedgeplatform.intel.com/edge-manage-docs/main/developer_guide/contributor_guide/index.html

## Overview

Edge Orchestrator Observability Tenant Controller is responsible for reconfiguration of the following components upon tenant creation and removal:

- [alerting-monitor] (via a dedicated gRPC management interface)
- [sre-exporter] (via a dedicated gRPC management interface)
- [edgenode-observability] (Grafana Mimir & Grafana Loki)

This service also provides tenant data via:

- a dedicated gRPC interface to [grafana-proxy] instances running in [edgenode-observability] and [orchestrator-observability]
- `project_metadata` metric exposed for scraping by [edgenode-observability] and [prometheus-agent] (which feeds it to [orchestrator-observability])

The multi-tenancy approach considers a `project` being a representation of a `tenant` as exposed by [tenancy-datamodel].

Read more about Edge Orchestrator Observability Tenant Controller in the [Documentation].

## Get Started

To set up the development environment and work on this project, follow the steps below.
All necessary tools will be installed using the `install-tools` target.
Note that `docker` and `asdf` must be installed beforehand.

### Install Tools

To install all the necessary tools needed for development the project, run:

```sh
make install-tools
```

### Build

To build the project, use the following command:

```sh
make build
```

### Lint

To lint the code and ensure it adheres to the coding standards, run:

```sh
make lint
```

### Test

To run the tests and verify the functionality of the project, use:

```sh
make test
```

### Docker Build

To build the Docker image for the project, run:

```sh
make docker-build
```

### Helm Build

To package the Helm chart for the project, use:

```sh
make helm-build
```

### Docker Push

To push the Docker image to the registry, run:

```sh
make docker-push
```

### Helm Push

To push the Helm chart to the repository, use:

```sh
make helm-push
```

### Kind All

To load the Docker image into a local Kind cluster, run:

```sh
make kind-all
```

### Proto

To generate code from protobuf definitions, use:

```sh
make proto
```

## Develop

It is recommended to develop the `observability-tenant-controller` application by deploying and testing it as a part of the Edge Orchestrator cluster.

The code of this project is maintained and released in CI using the `VERSION` file.
In addition, the chart is versioned with the same tag as the `VERSION` file.

This is mandatory to keep all chart versions and app versions coherent.

To bump the version, increment the version in the `VERSION` file and run the following command
(to set `version` and `appVersion` in the `Chart.yaml` automatically):

```sh
make helm-build
```

## Contribute

To learn how to contribute to the project, see the [Contributor's Guide].

## Community and Support

To learn more about the project, its community, and governance, visit the [Edge Orchestrator Community].

For support, start with [Troubleshooting] or [Contact us].

## License

Edge Orchestrator Observability Charts are licensed under [Apache 2.0 License].

Last Updated Date: {March 28, 2025}
