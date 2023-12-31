// SPDX-FileCopyrightText: 2023 Comcast Cable Communications Management, LLC
// SPDX-License-Identifier: Apache-2.0

package idock

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// DockerComposeFile sets the docker-compose file to use if docker-compose is
// used.
func DockerComposeFile(file string) Option {
	return optionFunc(func(c *IDock) {
		c.dockerComposeFile = file
	})
}

// RequireDockerTCPPorts ensures that the given ports are active before
// continuing on to the next step of starting the program.
func RequireDockerTCPPorts(ports ...int) Option {
	return optionFunc(func(c *IDock) {
		c.dockerTCPPorts = append(c.dockerTCPPorts, ports...)
	})
}

// DockerMaxWait sets the maximum amount of time to wait for the docker-compose
// programs to start.
//
// Defaults to 10 seconds.
func DockerMaxWait(d time.Duration) Option {
	return optionFunc(func(c *IDock) {
		if d < 0 {
			panic(fmt.Sprintf("dockerMaxWait must be >= 0: %s\n", d))
		}
		c.dockerMaxWait = d
	})
}

// DockerPullMaxWait sets the maximum amount of time to wait for the docker-compose
// programs to pull images.
//
// Defaults to 60 seconds.
func DockerPullMaxWait(d time.Duration) Option {
	return optionFunc(func(c *IDock) {
		if d < 0 {
			panic(fmt.Sprintf("dockerMaxPullWait must be >= 0: %s\n", d))
		}
		c.dockerMaxPullWait = d
	})
}

// AfterDocker is a function that is called after the docker-compose program
// has started but before the program is started.
func AfterDocker(f func(context.Context, *IDock)) Option {
	return optionFunc(func(c *IDock) {
		c.afterDocker = emptyAfter
		if f != nil {
			c.afterDocker = f
		}
	})
}

// Program is the function that is called to start the program.
func Program(f func()) Option {
	return optionFunc(func(c *IDock) {
		c.program = emptyProgram
		if f != nil {
			c.program = f
		}
	})
}

// RequireProgramTCPPorts ensures that the given ports are active before
// continuing on to the next step of running the tests.
func RequireProgramTCPPorts(ports ...int) Option {
	return optionFunc(func(c *IDock) {
		c.programTCPPorts = append(c.programTCPPorts, ports...)
	})
}

// ProgramMaxWait sets the maximum amount of time to wait for the program to
// start.
//
// The default value is 10 seconds.
func ProgramMaxWait(d time.Duration) Option {
	return optionFunc(func(c *IDock) {
		if d < 0 {
			panic(fmt.Sprintf("programMaxWait must be >= 0: %s\n", d))
		}
		c.programMaxWait = d
	})
}

// AfterProgram is a function that is called after the program has started but
// before the tests are run.
func AfterProgram(f func(context.Context, *IDock)) Option {
	return optionFunc(func(c *IDock) {
		c.afterProgram = emptyAfter
		if f != nil {
			c.afterProgram = f
		}
	})
}

// CleanupAttempts sets the number of times to attempt to cleanup the docker
// containers process before giving up and leaving any docker containers running.
// A value of 0 means do not attempt to cleanup the docker containers.  This is
// useful for debugging or speeding up tests.
//
// The default value is 3.
func CleanupAttempts(n int) Option {
	return optionFunc(func(c *IDock) {
		if n < 0 {
			panic(fmt.Sprintf("cleanupRetries must be >= 0: %d\n", n))
		}
		c.cleanupAttempts = n
	})
}

// Localhost sets the localhost address to use when connecting to the program.
//
// The default value is 'localhost'.
func Localhost(s string) Option {
	return optionFunc(func(c *IDock) {
		c.localhost = s
	})
}

// TCPPortMaxWait sets the maximum amount of time to wait while dialing a TCP
// port.
//
// The default value is 10 milliseconds.
func TCPPortMaxWait(d time.Duration) Option {
	return optionFunc(func(c *IDock) {
		if d < 0 {
			panic(fmt.Sprintf("tcpPortMaxWait must be >= 0: %s\n", d))
		}
		c.tcpPortMaxWait = d
	})
}

// Verbosity sets the verbosity level.
//
// The default value is 0.
func Verbosity(n int) Option {
	return optionFunc(func(c *IDock) {
		c.verbosity = n
	})
}

// VerbosityEnvarName sets the environment variable name to use for the
// verbosity level.
//
// The default value is IDOCK_VERBOSITY.
func VerbosityEnvarName(name string) Option {
	return optionFunc(func(c *IDock) {
		c.verbosityFlag = name
	})
}

// CleanupAttemptsEnvarName sets the environment variable name to use for the
// cleanup attempts.
//
// The default value is IDOCK_CLEANUP_RETRIES.
func CleanupAttemptsEnvarName(name string) Option {
	return optionFunc(func(c *IDock) {
		c.cleanupAttemptsFlag = name
	})
}

// DockerMaxWaitEnvarName sets the environment variable name to use for the
// docker max wait.
//
// The default value is IDOCK_DOCKER_MAX_WAIT.
func DockerMaxWaitEnvarName(name string) Option {
	return optionFunc(func(c *IDock) {
		c.dockerMaxWaitFlag = name
	})
}

// ProgramMaxWaitEnvarName sets the environment variable name to use for the
// program max wait.
//
// The default value is IDOCK_PROGRAM_MAX_WAIT.
func ProgramMaxWaitEnvarName(name string) Option {
	return optionFunc(func(c *IDock) {
		c.programMaxWaitFlag = name
	})
}

// DockerPullMaxWaitEnvarName sets the environment variable name to use for the
// docker pull max wait.
//
// The default value is IDOCK_DOCKER_PULL_MAX_WAIT.
func DockerPullMaxWaitEnvarName(name string) Option {
	return optionFunc(func(c *IDock) {
		c.dockerMaxPullWaitFlag = name
	})
}

// SkipDockerPullEnvarName sets the environment variable name to use for the
// skip docker pull flag.
//
// The default value is IDOCK_SKIP_DOCKER_PULL.
func SkipDockerPullEnvarName(name string) Option {
	return optionFunc(func(c *IDock) {
		c.skipDockerPullFlag = name
	})
}

// verbosity sets the verbosity level based on the environment variable.
func verbosity() Option {
	return optionFunc(func(c *IDock) {
		c.verbosity = envToInt(c.verbosityFlag, c.verbosity)
	})
}

// cleanupRetries sets the number of times to retry the cleanup process before
// giving up and leaving any docker containers running based on the environment
// variable.
func cleanupRetries() Option {
	return optionFunc(func(c *IDock) {
		c.cleanupAttempts = envToInt(c.cleanupAttemptsFlag, c.cleanupAttempts)
	})
}

// dockerMaxWait sets the maximum amount of time to wait for the docker-compose
// programs to start based on the environment variable.
func dockerMaxWait() Option {
	return optionFunc(func(c *IDock) {
		c.dockerMaxWait = envToDuration(c.dockerMaxWaitFlag, c.dockerMaxWait)
	})
}

// programMaxWait sets the maximum amount of time to wait for the program to
// start based on the environment variable.
func programMaxWait() Option {
	return optionFunc(func(c *IDock) {
		c.programMaxWait = envToDuration(c.programMaxWaitFlag, c.programMaxWait)
	})
}

// skipDockerPull sets the skipDockerPull flag based on the environment variable.
func skipDockerPull() Option {
	return optionFunc(func(c *IDock) {
		c.skipDockerPull = envToBool(c.skipDockerPullFlag, c.skipDockerPull)
	})
}

func envToDuration(name string, def time.Duration) time.Duration {
	s := strings.TrimSpace(os.Getenv(name))
	if s == "" {
		return def
	}
	d, err := time.ParseDuration(s)
	if err != nil {
		panic(fmt.Sprintf("%s having value '%s' must be a duration: %s\n",
			name, s, err))
	}
	return d
}

func envToInt(name string, def int) int {
	s := strings.TrimSpace(os.Getenv(name))
	if s == "" {
		return def
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		panic(fmt.Sprintf("%s having value '%s' must be an integer: %s\n",
			name, s, err))
	}
	return n
}

func envToBool(name string, def bool) bool {
	s := strings.TrimSpace(os.Getenv(name))
	if s == "" {
		return def
	}

	b, err := strconv.ParseBool(s)
	if err != nil {
		panic(fmt.Sprintf("%s having value '%s' must be a boolean: %s\n",
			name, s, err))
	}
	return b
}
