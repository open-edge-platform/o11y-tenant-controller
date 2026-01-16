# SPDX-FileCopyrightText: (C) 2025 Intel Corporation
# SPDX-License-Identifier: Apache-2.0

# Building environment
FROM golang:1.26rc2-alpine AS build

WORKDIR /workspace

RUN apk add --upgrade --no-cache make=~4 bash=~5

COPY . .

RUN make build

# Run tenant controller container
FROM alpine:3.23@sha256:865b95f46d98cf867a156fe4a135ad3fe50d2056aa3f25ed31662dff6da4eb62

COPY --from=build /workspace/build/observability-tenant-controller /observability-tenant-controller

RUN addgroup -S tcontroller && adduser -S tcontroller -G tcontroller
USER tcontroller

EXPOSE 9273 50051

ENTRYPOINT ["/observability-tenant-controller"]
