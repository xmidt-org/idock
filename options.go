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
func DockerMaxWait(d time.Duration) Option {
	return optionFunc(func(c *IDock) {
		if d < 0 {
			panic(fmt.Sprintf("dockerMaxWait must be >= 0: %s\n", d))
		}
		c.dockerMaxWait = d
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
// The default value is 2 seconds.
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
// The default value of 'localhost' is used.
func Localhost(s string) Option {
	return optionFunc(func(c *IDock) {
		c.localhost = s
	})
}

// TCPPortMaxWait sets the maximum amount of time to wait while dialing a TCP
// port.
// The default value is 2 seconds.
func TCPPortMaxWait(d time.Duration) Option {
	return optionFunc(func(c *IDock) {
		if d < 0 {
			panic(fmt.Sprintf("tcpPortMaxWait must be >= 0: %s\n", d))
		}
		c.tcpPortMaxWait = d
	})
}

// Verbosity sets the verbosity level.
func Verbosity(n int) Option {
	return optionFunc(func(c *IDock) {
		c.verbosity = n
	})
}

// VerbosityEnvarName sets the environment variable name to use for the
// verbosity level.
func VerbosityEnvarName(name string) Option {
	return optionFunc(func(c *IDock) {
		c.verbosityFlag = name
	})
}

// CleanupAttemptsEnvarName sets the environment variable name to use for the
// cleanup attempts.
func CleanupAttemptsEnvarName(name string) Option {
	return optionFunc(func(c *IDock) {
		c.cleanupAttemptsFlag = name
	})
}

// DockerMaxWaitEnvarName sets the environment variable name to use for the
// docker max wait.
func DockerMaxWaitEnvarName(name string) Option {
	return optionFunc(func(c *IDock) {
		c.dockerMaxWaitFlag = name
	})
}

// ProgramMaxWaitEnvarName sets the environment variable name to use for the
// program max wait.
func ProgramMaxWaitEnvarName(name string) Option {
	return optionFunc(func(c *IDock) {
		c.programMaxWaitFlag = name
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
