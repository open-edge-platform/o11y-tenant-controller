// SPDX-FileCopyrightText: (C) 2025 Intel Corporation
// SPDX-License-Identifier: Apache-2.0

package loki

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

const deleteEndpointResponse = `[
  {
    "request_id": "da329152",
    "start_time": 1,
    "end_time": 1727775678.942,
    "query": "{service_name!=\"\"}",
    "status": "processed",
    "created_at": 1727775678.942
  },
  {
    "request_id": "a0152cc0",
    "start_time": 1,
    "end_time": 1727775723.633,
    "query": "{service_name!=\"\"}",
    "status": "received",
    "created_at": 1727775723.634
  }
]`

const deleteEndpointResponseDone = `[
  {
    "request_id": "da329152",
    "start_time": 1,
    "end_time": 1727775678.942,
    "query": "{service_name!=\"\"}",
    "status": "processed",
    "created_at": 1727775678.942
  },
  {
    "request_id": "a0152cc0",
    "start_time": 1,
    "end_time": 1727775723.633,
    "query": "{service_name!=\"\"}",
    "status": "processed",
    "created_at": 1727775723.634
  }
]`

func TestCleanup(t *testing.T) {
	tests := map[string]struct {
		errorReturned        bool
		contextValue         bool
		flushHTTPCode        int
		deleteStatusHTTPCode int
		deleteHTTPCode       int
		deleteVerifyMode     utility.VerifyMode
	}{
		"Test cleanup path - no value in context": {
			errorReturned:        true,
			contextValue:         false,
			flushHTTPCode:        http.StatusNoContent,
			deleteStatusHTTPCode: http.StatusOK,
			deleteHTTPCode:       http.StatusNoContent,
			deleteVerifyMode:     utility.StrictMode,
		},
		"Test cleanup path - no error": {
			errorReturned:        false,
			contextValue:         true,
			flushHTTPCode:        http.StatusNoContent,
			deleteStatusHTTPCode: http.StatusOK,
			deleteHTTPCode:       http.StatusNoContent,
			deleteVerifyMode:     utility.StrictMode,
		},
		"Test cleanup path - flush error": {
			errorReturned:        true,
			contextValue:         true,
			flushHTTPCode:        http.StatusInternalServerError,
			deleteStatusHTTPCode: http.StatusOK,
			deleteHTTPCode:       http.StatusNoContent,
			deleteVerifyMode:     utility.StrictMode,
		},
		"Test cleanup path - deletion request error": {
			errorReturned:        true,
			contextValue:         true,
			flushHTTPCode:        http.StatusNoContent,
			deleteStatusHTTPCode: http.StatusOK,
			deleteHTTPCode:       http.StatusInternalServerError,
			deleteVerifyMode:     utility.StrictMode,
		},
		"Test cleanup path - deletion status error": {
			errorReturned:        true,
			contextValue:         true,
			flushHTTPCode:        http.StatusNoContent,
			deleteStatusHTTPCode: http.StatusInternalServerError,
			deleteHTTPCode:       http.StatusNoContent,
			deleteVerifyMode:     utility.StrictMode,
		},
		"Test cleanup path - deletion status loose mode": {
			errorReturned:        false,
			contextValue:         true,
			flushHTTPCode:        http.StatusNoContent,
			deleteStatusHTTPCode: http.StatusOK,
			deleteHTTPCode:       http.StatusNoContent,
			deleteVerifyMode:     utility.LooseMode,
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			var urlCfg config.Loki
			var returnStatus bool

			svr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/flush" {
					w.WriteHeader(test.flushHTTPCode)
				}
				if r.URL.Path == "/loki/api/v1/delete" {
					if returnStatus {
						w.WriteHeader(test.deleteStatusHTTPCode)
						fmt.Fprint(w, deleteEndpointResponseDone)
						return
					}
					w.WriteHeader(test.deleteHTTPCode)
					returnStatus = true
				}
			}))

			urlCfg.Write = svr.URL
			urlCfg.Backend = svr.URL
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
			var urlCfg config.Loki
			var svr *httptest.Server

			if test.server {
				svr = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if r.URL.Path == "/flush" {
						w.WriteHeader(test.svrResponseCode)
					}
				}))
				if !test.invalidURL {
					urlCfg.Write = svr.URL
				} else {
					urlCfg.Write = "invalid"
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
		"Test deleting logs request - server works and returning status 204": {
			server:          true,
			errorReturned:   false,
			invalidURL:      false,
			svrResponseCode: http.StatusNoContent,
		},
		"Test deleting logs request - invalid url": {
			server:          true,
			errorReturned:   true,
			invalidURL:      true,
			svrResponseCode: http.StatusNotFound,
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			var urlCfg config.Loki
			var svr *httptest.Server

			if test.server {
				svr = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if r.URL.Path == "/loki/api/v1/delete" {
						w.WriteHeader(test.svrResponseCode)
					}
				}))
				if !test.invalidURL {
					urlCfg.Backend = svr.URL
				} else {
					urlCfg.Backend = "invalid"
				}
				defer svr.Close()
			}
			if test.errorReturned {
				require.Error(t, deleteLogsRequest(t.Context(), urlCfg, "foo"), "Function doesn't return an error")
			} else {
				require.NoError(t, deleteLogsRequest(t.Context(), urlCfg, "foo"), "Function returned an error")
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
		"Test checking deletion progress - invalid url": {
			errorReturned:   true,
			invalidURL:      true,
			svrResponseCode: http.StatusNotFound,
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			urlCfg := config.Loki{
				PollingRate: 10 * time.Millisecond,
			}
			var done bool

			svr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/loki/api/v1/delete" {
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
				urlCfg.Backend = svr.URL
			} else {
				urlCfg.Backend = "invalid"
			}

			if test.errorReturned {
				require.Error(t, checkDeletionStatus(t.Context(), urlCfg, "foo"), "Function doesn't return an error")
			} else {
				require.NoError(t, checkDeletionStatus(t.Context(), urlCfg, "foo"), "Function returned an error")
			}
		})
	}

	t.Run("Test checking deletion progress - canceled context", func(t *testing.T) {
		var urlCfg config.Loki

		svr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/loki/api/v1/delete" {
				w.WriteHeader(http.StatusOK)
				fmt.Fprint(w, deleteEndpointResponse)
			}
		}))
		urlCfg.Backend = svr.URL
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

	t.Run("Test checking deletion progress - empty list as a response and then correct list", func(t *testing.T) {
		var done bool
		svr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/loki/api/v1/delete" {
				// Server responds OK but deletion request was not acknowledged.
				w.WriteHeader(http.StatusOK)
				if !done {
					fmt.Fprint(w, "[]")
				} else {
					fmt.Fprint(w, deleteEndpointResponseDone)
				}
				done = true
			}
		}))
		urlCfg := config.Loki{
			Backend:     svr.URL,
			PollingRate: 200 * time.Millisecond,
		}
		defer svr.Close()

		err := checkDeletionStatus(t.Context(), urlCfg, "foo")
		require.NoError(t, err, "Function returned an error")
	})
	t.Run("Test checking deletion progress - cannot unmarshal response", func(t *testing.T) {
		svr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/loki/api/v1/delete" {
				// Server responds OK but response body is corrupted.
				w.WriteHeader(http.StatusOK)
				fmt.Fprint(w, "{}")
			}
		}))
		urlCfg := config.Loki{
			Backend:     svr.URL,
			PollingRate: 200 * time.Millisecond,
		}
		defer svr.Close()

		err := checkDeletionStatus(t.Context(), urlCfg, "foo")
		require.ErrorContains(t, err, "failed to unmarshal")
	})
}
