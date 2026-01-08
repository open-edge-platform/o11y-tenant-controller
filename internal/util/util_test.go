// SPDX-FileCopyrightText: (C) 2025 Intel Corporation
// SPDX-License-Identifier: Apache-2.0

package util_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/open-edge-platform/o11y-tenant-controller/internal/util"

	"github.com/stretchr/testify/require"
)

func TestSleepWithContext(t *testing.T) {
	startTime := time.Now()
	setSleepTime := 10 * time.Millisecond
	timeout := 5 * time.Millisecond

	err := util.SleepWithContext(t.Context(), setSleepTime)
	require.GreaterOrEqual(t, time.Since(startTime), setSleepTime, "Sleep time is shorter than expected")
	require.NoError(t, err, "Function returned an error")

	startTime = time.Now()
	ctx, cancel := context.WithTimeout(t.Context(), timeout)
	defer cancel()

	err = util.SleepWithContext(ctx, setSleepTime)
	actualSleepTime := time.Since(startTime)
	require.Less(t, actualSleepTime, setSleepTime, "Sleep time is longer than expected")
	require.GreaterOrEqual(t, actualSleepTime, timeout, "Sleep time shorter than timeout")
	require.Error(t, err, "Function didn't return an error")
	require.EqualError(t, err, context.DeadlineExceeded.Error(), "Error different than expected")
}

func TestPostReq(t *testing.T) {
	tests := map[string]struct {
		server          bool
		errorReturned   bool
		svrResponseCode int
	}{
		"Test post request - server doesn't work": {
			server:        false,
			errorReturned: true,
		},
		"Test post request - server works and returning status 200": {
			server:          true,
			errorReturned:   false,
			svrResponseCode: http.StatusOK,
		},
		"Test post request - server works and returning status 204": {
			server:          true,
			errorReturned:   false,
			svrResponseCode: http.StatusNoContent,
		},
		"Test post request - server works and returning status 404": {
			server:          true,
			errorReturned:   true,
			svrResponseCode: http.StatusNotFound,
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			var srvURL string
			var svr *httptest.Server

			if test.server {
				svr = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					w.WriteHeader(test.svrResponseCode)
				}))
				srvURL = svr.URL
				defer svr.Close()
			}
			if test.errorReturned {
				require.Error(t, util.PostReq(t.Context(), srvURL, "foo"), "Function doesn't return an error")
			} else {
				require.NoError(t, util.PostReq(t.Context(), srvURL, "foo"), "Function returned an error")
			}
		})
	}
}

func TestGetReq(t *testing.T) {
	tests := map[string]struct {
		server          bool
		errorReturned   bool
		svrResponseCode int
	}{
		"Test post request - server doesn't work": {
			server:        false,
			errorReturned: true,
		},
		"Test post request - server works and returning status 200": {
			server:          true,
			errorReturned:   false,
			svrResponseCode: http.StatusOK,
		},
		"Test post request - server works and returning status 204": {
			server:          true,
			errorReturned:   false,
			svrResponseCode: http.StatusNoContent,
		},
		"Test post request - server works and returning status 404": {
			server:          true,
			errorReturned:   true,
			svrResponseCode: http.StatusNotFound,
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			var srvURL string
			var svr *httptest.Server

			if test.server {
				svr = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					w.WriteHeader(test.svrResponseCode)
				}))
				srvURL = svr.URL
				defer svr.Close()
			}

			_, err := util.GetReq(t.Context(), srvURL, "foo")
			if test.errorReturned {
				require.Error(t, err, "Function doesn't return an error")
			} else {
				require.NoError(t, err, "Function returned an error")
			}
		})
	}
}
