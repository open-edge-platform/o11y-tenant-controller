// SPDX-FileCopyrightText: (C) 2025 Intel Corporation
// SPDX-License-Identifier: Apache-2.0

package mimir

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"

	"github.com/open-edge-platform/o11y-tenant-controller/internal/config"
	"github.com/open-edge-platform/o11y-tenant-controller/internal/util"
)

const deleteEndpointResponse = `{
	"tenant_id": "foo",
	"blocks_deleted": false
}`

const deleteEndpointResponseDone = `{
	"tenant_id": "foo",
	"blocks_deleted": true
}`

func TestCleanup(t *testing.T) {
	tests := map[string]struct {
		errorReturned              bool
		contextValue               bool
		flushHTTPcode              int
		deleteTenantHTTPCode       int
		deleteTenantStatusHTTPCode int
		deleteVerifyMode           utility.VerifyMode
	}{
		"Test cleanup path - no value in context": {
			errorReturned:              true,
			contextValue:               false,
			flushHTTPcode:              http.StatusNoContent,
			deleteTenantHTTPCode:       http.StatusNoContent,
			deleteTenantStatusHTTPCode: http.StatusOK,
			deleteVerifyMode:           utility.StrictMode,
		},
		"Test cleanup path - no error": {
			errorReturned:              false,
			contextValue:               true,
			flushHTTPcode:              http.StatusNoContent,
			deleteTenantHTTPCode:       http.StatusNoContent,
			deleteTenantStatusHTTPCode: http.StatusOK,
			deleteVerifyMode:           utility.StrictMode,
		},
		"Test cleanup path - flush error": {
			errorReturned:              true,
			contextValue:               true,
			flushHTTPcode:              http.StatusInternalServerError,
			deleteTenantHTTPCode:       http.StatusNoContent,
			deleteTenantStatusHTTPCode: http.StatusOK,
			deleteVerifyMode:           utility.StrictMode,
		},
		"Test cleanup path - deletion tenant error": {
			errorReturned:              true,
			contextValue:               true,
			flushHTTPcode:              http.StatusNoContent,
			deleteTenantHTTPCode:       http.StatusInternalServerError,
			deleteTenantStatusHTTPCode: http.StatusOK,
			deleteVerifyMode:           utility.StrictMode,
		},
		"Test cleanup path - deletion tenant status error": {
			errorReturned:              true,
			contextValue:               true,
			flushHTTPcode:              http.StatusNoContent,
			deleteTenantHTTPCode:       http.StatusNoContent,
			deleteTenantStatusHTTPCode: http.StatusInternalServerError,
			deleteVerifyMode:           utility.StrictMode,
		},
		"Test cleanup path - deletion status check disabled": {
			errorReturned:              false,
			contextValue:               true,
			flushHTTPcode:              http.StatusNoContent,
			deleteTenantHTTPCode:       http.StatusNoContent,
			deleteTenantStatusHTTPCode: http.StatusInternalServerError,
			deleteVerifyMode:           utility.LooseMode,
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			var urlCfg config.Mimir

			svr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/ingester/flush" {
					w.WriteHeader(test.flushHTTPcode)
				}
				if r.URL.Path == "/compactor/delete_tenant" {
					w.WriteHeader(test.deleteTenantHTTPCode)
				}
				if r.URL.Path == "/compactor/delete_tenant_status" {
					w.WriteHeader(test.deleteTenantStatusHTTPCode)
					fmt.Fprint(w, deleteEndpointResponseDone)
				}
			}))

			urlCfg.Ingester = svr.URL
			urlCfg.Compactor = svr.URL
			urlCfg.DeleteVerifyMode = test.deleteVerifyMode
			defer svr.Close()

			ctx := t.Context()
			if test.contextValue {
				ctx = context.WithValue(ctx, utility.ContextKeyTenantID, "foo")
			}

			err := CleanupTenant(ctx, urlCfg)
			if test.errorReturned {
				require.Error(t, err, "Function doesn't return an error")
			} else {
				require.NoError(t, err, "Function returned an error")
			}
		})
	}
}

func TestFlushIngesters(t *testing.T) {
	tests := map[string]struct {
		server          bool
		errorReturned   bool
		invalidURL      bool
		svrResponseCode int
	}{
		"Test flushing ingesters - server works and returning status 204": {
			server:          true,
			errorReturned:   false,
			invalidURL:      false,
			svrResponseCode: http.StatusNoContent,
		},
		"Test flushing ingesters - invalid url": {
			server:          true,
			errorReturned:   true,
			invalidURL:      true,
			svrResponseCode: http.StatusNotFound,
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			var urlCfg config.Mimir
			var svr *httptest.Server

			if test.server {
				svr = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if r.URL.Path == "/ingester/flush" {
						w.WriteHeader(test.svrResponseCode)
					}
				}))
				if !test.invalidURL {
					urlCfg.Ingester = svr.URL
				} else {
					urlCfg.Ingester = "invalid"
				}
				defer svr.Close()
			}
			if test.errorReturned {
				require.Error(t, flushIngesters(t.Context(), urlCfg, "foo"), "Function doesn't return an error")
			} else {
				require.NoError(t, flushIngesters(t.Context(), urlCfg, "foo"), "Function returned an error")
			}
		})
	}
}

func TestMakeDeleteReq(t *testing.T) {
	tests := map[string]struct {
		server          bool
		errorReturned   bool
		invalidURL      bool
		svrResponseCode int
	}{
		"Test deleting metrics request - server works and returning status 204": {
			server:          true,
			errorReturned:   false,
			invalidURL:      false,
			svrResponseCode: http.StatusNoContent,
		},
		"Test deleting metrics request - invalid url": {
			server:          true,
			errorReturned:   true,
			invalidURL:      true,
			svrResponseCode: http.StatusNotFound,
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			var urlCfg config.Mimir
			var svr *httptest.Server

			if test.server {
				svr = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if r.URL.Path == "/compactor/delete_tenant" {
						w.WriteHeader(test.svrResponseCode)
					}
				}))
				if !test.invalidURL {
					urlCfg.Compactor = svr.URL
				} else {
					urlCfg.Compactor = "invalid"
				}
				defer svr.Close()
			}
			if test.errorReturned {
				require.Error(t, deleteMetricsRequest(t.Context(), urlCfg, "foo"), "Function doesn't return an error")
			} else {
				require.NoError(t, deleteMetricsRequest(t.Context(), urlCfg, "foo"), "Function returned an error")
			}
		})
	}
}

func TestCheckDeletionStatus(t *testing.T) {
	tests := map[string]struct {
		errorReturned   bool
		invalidURL      bool
		svrResponseCode int
	}{
		"Test checking deletion progress - server works and returning status 204": {
			errorReturned:   false,
			invalidURL:      false,
			svrResponseCode: http.StatusOK,
		},
		"Test deleting deletion progress - invalid url": {
			errorReturned:   true,
			invalidURL:      true,
			svrResponseCode: http.StatusNotFound,
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			urlCfg := config.Mimir{
				PollingRate: time.Second,
			}
			var done bool

			svr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/compactor/delete_tenant_status" {
					w.WriteHeader(test.svrResponseCode)
					if !done {
						fmt.Fprint(w, deleteEndpointResponse)
					} else {
						fmt.Fprint(w, deleteEndpointResponseDone)
					}
					done = true
				}
			}))
			defer svr.Close()

			if !test.invalidURL {
				urlCfg.Compactor = svr.URL
			} else {
				urlCfg.Compactor = "invalid"
			}

			if test.errorReturned {
				require.Error(t, checkDeletionStatus(t.Context(), urlCfg, "foo"), "Function doesn't return an error")
			} else {
				require.NoError(t, checkDeletionStatus(t.Context(), urlCfg, "foo"), "Function returned an error")
			}
		})
	}

	t.Run("Test checking deletion progress - canceled context", func(t *testing.T) {
		urlCfg := config.Mimir{
			PollingRate: 10 * time.Millisecond,
		}

		svr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/compactor/delete_tenant_status" {
				w.WriteHeader(http.StatusOK)
				fmt.Fprint(w, deleteEndpointResponse)
			}
		}))
		urlCfg.Compactor = svr.URL
		urlCfg.PollingRate = 200 * time.Millisecond
		defer svr.Close()

		ctx, cancel := context.WithTimeout(t.Context(), time.Second)
		defer cancel()
		g, ctx := errgroup.WithContext(ctx)

		g.Go(func() error {
			return checkDeletionStatus(ctx, urlCfg, "foo")
		})

		err := g.Wait()
		require.Error(t, err, "Function doesn't return an error")
	})

	t.Run("Test checking deletion progress - cannot unmarshal response", func(t *testing.T) {
		svr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/compactor/delete_tenant_status" {
				// Server responds OK but response body is corrupted.
				w.WriteHeader(http.StatusOK)
				fmt.Fprint(w, "[]]")
			}
		}))
		urlCfg := config.Mimir{
			Compactor:   svr.URL,
			PollingRate: 200 * time.Millisecond,
		}
		defer svr.Close()

		err := checkDeletionStatus(t.Context(), urlCfg, "foo")
		require.ErrorContains(t, err, "failed to unmarshal")
	})
}
