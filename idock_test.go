// SPDX-FileCopyrightText: 2023 Comcast Cable Communications Management, LLC
// SPDX-License-Identifier: Apache-2.0

package idock

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var unknownErr = errors.New("unknown error")

func TestIDock_startDocker(t *testing.T) {
	var noDockerCompose bool
	tests := []struct {
		description string
		have        IDock
		makeServer  bool
		expectErr   error
	}{
		{
			description: "no docker-compose file",
		}, {
			description: "a docker-compose file, but not enought time",
			have: IDock{
				dockerComposeFile: "docker-compose.yml",
				dockerMaxWait:     1 * time.Nanosecond,
			},
			expectErr: unknownErr,
		}, {
			description: "success, no verbosity",
			have: IDock{
				tcpPortMaxWait:    10 * time.Millisecond,
				dockerComposeFile: "docker-compose.yml",
				dockerMaxWait:     15 * time.Second,
				dockerMaxPullWait: 60 * time.Second,
				localhost:         "localhost",
				dockerTCPPorts:    []int{7999, 7998, 7997},
			},
		}, {
			description: "success, verbosity",
			have: IDock{
				verbosity:         99,
				skipDockerPull:    true,
				tcpPortMaxWait:    10 * time.Millisecond,
				dockerComposeFile: "docker-compose.yml",
				dockerMaxWait:     15 * time.Second,
				dockerMaxPullWait: 60 * time.Second,
				localhost:         "localhost",
				dockerTCPPorts:    []int{7999},
			},
		}, {
			description: "success, verbosity",
			have: IDock{
				verbosity:         99,
				tcpPortMaxWait:    10 * time.Millisecond,
				dockerComposeFile: "docker-compose.yml",
				dockerMaxWait:     15 * time.Second,
				dockerMaxPullWait: 60 * time.Second,
				localhost:         "localhost",
				dockerTCPPorts:    []int{7999},
			},
		}, {
			description: "a docker-compose file, but not enough ports",
			have: IDock{
				tcpPortMaxWait:    10 * time.Millisecond,
				dockerComposeFile: "docker-compose.yml",
				dockerMaxWait:     3 * time.Second,
				dockerMaxPullWait: 60 * time.Second,
				localhost:         "localhost",
				dockerTCPPorts:    []int{7999, 9999},
			},
			expectErr: unknownErr,
		},
	}
	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			assert := assert.New(t)
			//require := require.New(t)

			err := tc.have.startDocker(context.Background())

			if tc.have.noDockerCompose {
				noDockerCompose = true
			}
			switch tc.expectErr {
			case nil:
				assert.NoError(err)
			case unknownErr:
				assert.Error(err)
			default:
				assert.ErrorIs(err, tc.expectErr)
			}
		})
	}

	clean := IDock{
		dockerComposeFile: "docker-compose.yml",
		dockerStarted:     true,
		verbosity:         99,
		noDockerCompose:   noDockerCompose,
	}
	clean.Stop(5)
}

func TestIDock_startProgram(t *testing.T) {
	tests := []struct {
		description string
		have        IDock
		makeServer  bool
		expectErr   error
	}{
		{
			description: "program returns",
			have: IDock{
				program:        func() {},
				programMaxWait: time.Millisecond * 100,
			},
		}, {
			description: "program never returns",
			have: IDock{
				tcpPortMaxWait: 10 * time.Millisecond,
				program: func() {
					for {
						time.Sleep(time.Second)
					}
				},
				programMaxWait: time.Millisecond * 100,
			},
		}, {
			description: "program never returns, ports are open",
			have: IDock{
				tcpPortMaxWait: 10 * time.Millisecond,
				program: func() {
					for {
						time.Sleep(time.Second)
					}
				},
				programMaxWait: time.Millisecond * 100,
			},
			makeServer: true,
		}, {
			description: "program never returns, not enough ports are open",
			have: IDock{
				tcpPortMaxWait: 10 * time.Millisecond,
				program: func() {
					for {
						time.Sleep(time.Second)
					}
				},
				programTCPPorts: []int{51},
				programMaxWait:  time.Millisecond * 100,
			},
			makeServer: true,
			expectErr:  unknownErr,
		},
	}
	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			assert := assert.New(t)
			require := require.New(t)

			if tc.makeServer {
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusOK)
					_, _ = w.Write([]byte("Mock response"))
				}))
				defer server.Close()

				u, err := url.Parse(server.URL)
				require.NoError(err)
				authority := u.Hostname()
				port, err := strconv.Atoi(u.Port())
				require.NoError(err)

				tc.have.localhost = authority
				tc.have.programTCPPorts = append(tc.have.programTCPPorts, port)
			}

			err := tc.have.startProgram(context.Background())

			switch tc.expectErr {
			case nil:
				assert.NoError(err)
			case unknownErr:
				assert.Error(err)
			default:
				assert.ErrorIs(err, tc.expectErr)
			}
		})
	}
}

func TestIDock_isPortOpen(t *testing.T) {
	tests := []struct {
		description string
		wait        time.Duration
		expectErr   error
	}{
		{
			description: "port is not open",
			wait:        100 * time.Millisecond,
			expectErr:   unknownErr,
		}, {
			description: "port is open",
			wait:        100 * time.Millisecond,
		},
	}
	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			assert := assert.New(t)
			require := require.New(t)

			authority := "localhost"
			port := 51 // probably not used

			if tc.expectErr == nil {
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusOK)
					_, _ = w.Write([]byte("Mock response"))
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

			err := c.isPortOpen(ctx, port)

			switch tc.expectErr {
			case nil:
				assert.NoError(err)

			case unknownErr:
				assert.Error(err)
			default:
				assert.ErrorIs(err, tc.expectErr)
			}
		})
	}
}

func TestNewAndOptions(t *testing.T) {
	tests := []struct {
		description string
		opts        []Option
		env         map[string]string
		validate    func(*assert.Assertions, *IDock)
	}{
		{
			description: "All the options",
			opts: []Option{
				TCPPortMaxWait(15 * time.Millisecond),
				Localhost("localhost"),

				DockerComposeFile("docker-compose.yml"),
				DockerMaxWait(5 * time.Second),
				RequireDockerTCPPorts(7999),

				AfterDocker(func(context.Context, *IDock) {}),

				Program(func() {}),
				ProgramMaxWait(5 * time.Second),
				RequireProgramTCPPorts(7999),

				AfterProgram(func(context.Context, *IDock) {}),
				CleanupAttempts(3),
				Verbosity(5),
			},
			env: map[string]string{
				"IDOCK_VERBOSITY":        "2",
				"IDOCK_DOCKER_MAX_WAIT":  "15s",
				"IDOCK_PROGRAM_MAX_WAIT": "15s",
				"IDOCK_CLEANUP_ATTEMPTS": "5",
			},
			validate: func(assert *assert.Assertions, c *IDock) {
				if !assert.NotNil(c) {
					return
				}
				assert.Equal(15*time.Millisecond, c.tcpPortMaxWait)
				assert.Equal("localhost", c.localhost)

				assert.Equal("docker-compose.yml", c.dockerComposeFile)
				assert.Equal(15*time.Second, c.dockerMaxWait)
				assert.Equal([]int{7999}, c.dockerTCPPorts)

				assert.NotNil(c.afterDocker)

				assert.NotNil(c.program)
				assert.Equal(15*time.Second, c.programMaxWait)
				assert.Equal([]int{7999}, c.programTCPPorts)

				assert.NotNil(c.afterProgram)
				assert.Equal(5, c.cleanupAttempts)
				assert.Equal(2, c.Verbosity())
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			assert := assert.New(t)

			for k, v := range tc.env {
				t.Setenv(k, v)
			}
			c := New(tc.opts...)
			tc.validate(assert, c)
		})
	}
}

func TestRun(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("Mock response"))
	}))
	defer server.Close()

	require := require.New(t)
	u, err := url.Parse(server.URL)
	require.NoError(err)
	authority := u.Hostname()
	port, err := strconv.Atoi(u.Port())
	require.NoError(err)

	var afterDockerCallCount int
	var programCallCount int
	var afterProgramCallCount int

	tests := []struct {
		description string
		opts        []Option
		expectErr   error
	}{
		{
			description: "Many options",
			opts: []Option{
				TCPPortMaxWait(15 * time.Millisecond),
				Localhost(authority),

				DockerComposeFile("docker-compose.yml"),
				DockerMaxWait(15 * time.Second),
				RequireDockerTCPPorts(7999),

				AfterDocker(func(context.Context, *IDock) {
					afterDockerCallCount++
				}),

				Program(func() {
					programCallCount++
				}),
				ProgramMaxWait(15 * time.Second),
				RequireProgramTCPPorts(port),

				AfterProgram(func(context.Context, *IDock) {
					afterProgramCallCount++
				}),
				CleanupAttempts(5),
			},
		}, {
			description: "fail starting docker",
			opts: []Option{
				TCPPortMaxWait(15 * time.Millisecond),
				Localhost(authority),

				DockerComposeFile("docker-compose.yml"),
				DockerMaxWait(15 * time.Millisecond),
				RequireDockerTCPPorts(8001),
				CleanupAttempts(5),
			},
			expectErr: unknownErr,
		}, {
			description: "fail starting program",
			opts: []Option{
				TCPPortMaxWait(15 * time.Millisecond),
				Localhost(authority),

				DockerComposeFile("docker-compose.yml"),
				DockerMaxWait(15 * time.Second),
				RequireDockerTCPPorts(7999),

				ProgramMaxWait(100 * time.Millisecond),
				RequireProgramTCPPorts(port, 99),

				CleanupAttempts(5),
			},
			expectErr: unknownErr,
		},
	}

	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			assert := assert.New(t)

			c := New(tc.opts...)
			err := c.Start()

			switch tc.expectErr {
			case nil:
				c.Stop()
				assert.NoError(err)

			case unknownErr:
				assert.Error(err)
			default:
				assert.ErrorIs(err, tc.expectErr)
			}
		})
	}
}
