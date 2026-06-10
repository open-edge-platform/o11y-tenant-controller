# SPDX-FileCopyrightText: (C) 2026 Intel Corporation
# SPDX-License-Identifier: Apache-2.0

# Building environment
FROM golang:1.26.3-alpine3.23@sha256:91eda9776261207ea25fd06b5b7fed8d397dd2c0a283e77f2ab6e91bfa71079d AS build

WORKDIR /workspace

RUN apk add --upgrade --no-cache make=~4 bash=~5

COPY . .

RUN make build

# Run tenant controller container
FROM alpine:3.24@sha256:8ddefa941e689fc29abcdeb8dae3b3c6d139cc08ce9a52633931160701770685

# Upgrade zlib to fix CVE-2026-22184
RUN apk add --upgrade --no-cache "zlib>=1.3.2-r0"

COPY --from=build /workspace/build/observability-tenant-controller /observability-tenant-controller

RUN addgroup -S tcontroller && adduser -S tcontroller -G tcontroller
USER tcontroller

EXPOSE 9273 50051

ENTRYPOINT ["/observability-tenant-controller"]
