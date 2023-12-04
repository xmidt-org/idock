// SPDX-FileCopyrightText: 2023 Comcast Cable Communications Management, LLC
// SPDX-License-Identifier: Apache-2.0

package idock

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIDock_safelyWrap(t *testing.T) {
	tests := []struct {
		description string
		have        IDock
		expect      bool
	}{
		{
			description: "program returns",
			have: IDock{
				program:        func() {},
				programMaxWait: time.Second,
			},
			expect: true,
		}, {
			description: "program panics",
			have: IDock{
				program:        func() { panic("program panics") },
				programMaxWait: time.Second,
			},
		}, {
			expect:      true,
			description: "program never returns",
			have: IDock{
				program: func() {
					for {
						time.Sleep(time.Second)
					}
				},
				programMaxWait: time.Second,
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			assert := assert.New(t)
			got := tc.have.safelyWrap()
			assert.Equal(tc.expect, got)
		})
	}
}

func TestIDock_isPortOpen(t *testing.T) {
	tests := []struct {
		description string
		wait        time.Duration
		expect      bool
	}{
		{
			description: "port is not open",
			wait:        100 * time.Millisecond,
		}, {
			description: "port is open",
			wait:        100 * time.Millisecond,
			expect:      true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			assert := assert.New(t)
			require := require.New(t)

			authority := "localhost"
			port := 51 // probably not used

			if tc.expect {
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusOK)
					w.Write([]byte("Mock response"))
				}))
				defer server.Close()

				u, err := url.Parse(server.URL)
				require.NoError(err)
				authority = u.Hostname()
				port, err = strconv.Atoi(u.Port())
				require.NoError(err)
			}
			c := &IDock{
				localhost: authority,
			}

			ctx, cancel := context.WithTimeout(context.Background(), tc.wait)
			defer cancel()

			assert.Equal(tc.expect, c.isPortOpen(ctx, port))
		})
	}
}

func TestIDock_wait(t *testing.T) {
	tests := []struct {
		description string
		ports       []int
		done        chan struct{}
		expectedErr error
	}{
		{
			description: "",
			// TODO: Add test cases.
		},
	}
	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			assert := assert.New(t)
			require := require.New(t)
			c := &IDock{
				tcpPortMaxWait:    tc.fields.tcpPortMaxWait,
				dockerComposeFile: tc.fields.dockerComposeFile,
				dockerTCPPorts:    tc.fields.dockerTCPPorts,
				dockerMaxWait:     tc.fields.dockerMaxWait,
				afterDocker:       tc.fields.afterDocker,
				program:           tc.fields.program,
				programTCPPorts:   tc.fields.programTCPPorts,
				programMaxWait:    tc.fields.programMaxWait,
				afterProgram:      tc.fields.afterProgram,
				cleanupRetries:    tc.fields.cleanupRetries,
				localhost:         tc.fields.localhost,
				verbosity:         tc.fields.verbosity,
				noDockerCompose:   tc.fields.noDockerCompose,
			}

			err := c.wait(tc.ports, tc.done)

			assert.ErrorIs(err, tc.expectedErr)
		})
	}
}
