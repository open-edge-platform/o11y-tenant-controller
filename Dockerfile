# SPDX-FileCopyrightText: (C) 2025 Intel Corporation
# SPDX-License-Identifier: Apache-2.0

# Building environment
FROM golang:1.24.3-alpine@sha256:b4f875e650466fa0fe62c6fd3f02517a392123eea85f1d7e69d85f780e4db1c1 AS build

WORKDIR /workspace

RUN apk add --upgrade --no-cache make=~4 bash=~5

COPY . .

RUN make build

# Run tenant controller container
FROM alpine:3.22@sha256:8a1f59ffb675680d47db6337b49d22281a139e9d709335b492be023728e11715

COPY --from=build /workspace/build/observability-tenant-controller /observability-tenant-controller

RUN addgroup -S tcontroller && adduser -S tcontroller -G tcontroller
USER tcontroller

EXPOSE 9273 50051

ENTRYPOINT ["/observability-tenant-controller"]
