<!--
SPDX-FileCopyrightText: (C) 2025 Intel Corporation
SPDX-License-Identifier: Apache-2.0
-->

# Observability Tenant Controller Changelog

## [v0.5.43](https://github.com/open-edge-platform/o11y-tenant-controller/tree/v0.5.43)

- Initial release
- Application `observability-tenant-controller` added:
  - Monitors [tenancy-datamodel](https://github.com/open-edge-platform/orch-utils/tree/main/tenancy-datamodel) for project (tenant) creation and removal events
  - Reconfigures [alerting-monitor]( https://github.com/open-edge-platform/o11y-alerting-monitor), [edgenode-observability](https://github.com/open-edge-platform/o11y-charts/tree/main/charts/edgenode-observability) and [sre-exporter](https://github.com/open-edge-platform/o11y-sre-exporter) (optionally)
  - Streams current project data via `gRPC` to [grafana-proxy](https://github.com/open-edge-platform/o11y-charts/tree/main/apps/grafana-proxy) instances
  - Exposes project details via `project_metadata` metric in [Prometheus](https://prometheus.io/docs/concepts/data_model/) format
  - Enables `strict` or `loose` tenant data removal verification options
