// SPDX-FileCopyrightText: 2023 Comcast Cable Communications Management, LLC
// SPDX-License-Identifier: Apache-2.0

package idock

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"sort"
	"sync"
	"time"
)

const (
	// IDOCK_VERBOSITY_FLAG is the default environment variable that controls
	// the verbosity of the output.
	VERBOSITY_FLAG = "IDOCK_VERBOSITY"

	// DOCKER_MAX_PULL_WAIT_FLAG is the default environment variable that
	// controls how long to wait for the docker-compose program to pull images.
	DOCKER_MAX_PULL_WAIT_FLAG = "IDOCK_DOCKER_MAX_PULL_WAIT"

	// DOCKER_MAX_WAIT_FLAG is the default environment variable that controls
	// how long to wait for the docker-compose program to start.
	DOCKER_MAX_WAIT_FLAG = "IDOCK_DOCKER_MAX_WAIT"

	// PROGRAM_MAX_WAIT_FLAG is the default environment variable that controls
	// how long to wait for the program to start.
	PROGRAM_MAX_WAIT_FLAG = "IDOCK_PROGRAM_MAX_WAIT"

	// CLEANUP_ATTEMPTS_FLAG is the default environment variable that controls
	// how many times to retry the cleanup process.
	CLEANUP_ATTEMPTS_FLAG = "IDOCK_CLEANUP_ATTEMPTS"
)

var (
	errTimedOut = fmt.Errorf("timed out")
)

// IDock is the main struct for the idock package.
type IDock struct {
	// env variable names
	verbosityFlag         string
	dockerMaxPullWaitFlag string
	dockerMaxWaitFlag     string
	programMaxWaitFlag    string
	cleanupAttemptsFlag   string

	tcpPortMaxWait    time.Duration
	dockerComposeFile string
	dockerTCPPorts    []int
	dockerMaxWait     time.Duration
	dockerMaxPullWait time.Duration
	afterDocker       func(context.Context, *IDock)
	program           func()
	programTCPPorts   []int
	programMaxWait    time.Duration
	afterProgram      func(context.Context, *IDock)
	cleanupAttempts   int
	localhost         string
	verbosity         int
	noDockerCompose   bool
	dockerStarted     bool
}

// Option is an option interface for the IDock struct.
type Option interface {
	apply(*IDock)
}

type optionFunc func(*IDock)

func (f optionFunc) apply(c *IDock) {
	f(c)
}

func emptyProgram()                      {}
func emptyAfter(context.Context, *IDock) {}

// New creates a new IDock struct with the given options.
func New(opts ...Option) *IDock {
	var c IDock

	defaults := []Option{
		VerbosityEnvarName(VERBOSITY_FLAG),
		CleanupAttemptsEnvarName(CLEANUP_ATTEMPTS_FLAG),
		DockerMaxWaitEnvarName(DOCKER_MAX_WAIT_FLAG),
		ProgramMaxWaitEnvarName(PROGRAM_MAX_WAIT_FLAG),
		DockerPullMaxWaitEnvarName(DOCKER_MAX_PULL_WAIT_FLAG),
		TCPPortMaxWait(10 * time.Millisecond),
		DockerMaxWait(10 * time.Second),
		DockerPullMaxWait(60 * time.Second),
		ProgramMaxWait(10 * time.Second),
		AfterDocker(nil),
		Program(nil),
		AfterProgram(nil),
		Localhost("localhost"),
		CleanupAttempts(3),
	}

	opts = append(defaults, opts...)
	opts = append(opts, []Option{
		verbosity(),
		cleanupRetries(),
		dockerMaxWait(),
		programMaxWait(),
	}...)

	for _, opt := range opts {
		opt.apply(&c)
	}

	return &c
}

// Verbosity gets the verbosity level.  The verbosity level is set by the
// IDOCK_VERBOSITY environment variable, or by the Verbosity option.  The
// environment variable takes precedence over the option.  The default value is
// 0.
func (c *IDock) Verbosity() int {
	return c.verbosity
}

// Start starts the docker-compose services and the program.  It waits for the
// docker-compose services and the program to start before returning.  If the
// docker-compose services or the program fail to start, then an error is
// returned.
func (c *IDock) Start() error {
	ctx := context.Background()
	err := c.startDocker(ctx)
	if err != nil {
		c.logf(0, "docker startup failed: %s\n", err)
		return err
	}

	if c.afterDocker != nil {
		start := time.Now()
		c.afterDocker(ctx, c)
		end := time.Now()
		c.logf(1, "customization after docker took %s\n", end.Sub(start))
	}

	err = c.startProgram(ctx)
	if err != nil {
		c.logf(0, "program startup failed: %s\n", err)
		return err
	}

	if c.afterProgram != nil {
		start := time.Now()
		c.afterProgram(ctx, c)
		end := time.Now()
		c.logf(1, "customization after program took %s\n", end.Sub(start))
	}

	return nil
}

// Stop stops the docker-compose services and cleans up any docker containers
// that were started.  If force is set to a value greater than 0, then the
// cleanup process is attempted that many times before giving up, overwriting
// the value set by the CleanupAttempts option or the IDOCK_CLEANUP_ATTEMPTS
// environment variable.
func (c *IDock) Stop(force ...int) {
	if len(force) > 0 && force[0] > 0 {
		c.cleanupAttempts = force[0]
	}
	c.cleanup()
}

func (c *IDock) startDocker(ctx context.Context) error {
	if c.dockerComposeFile == "" {
		return nil
	}

	verbose := c.verbosity > 1

	cmd, err := dockerCompose(ctx, verbose, "-f", c.dockerComposeFile, "pull")
	if err != nil {
		return fmt.Errorf("error pulling docker images: %w", err)
	}

	cmd.WaitDelay = c.dockerMaxPullWait
	dockerPullStart := time.Now()
	c.logf(1, "Waiting for docker images to pull...\n")
	err = cmd.Start()
	if err != nil {
		return fmt.Errorf("error pulling docker images: %w", err)
	}

	err = cmd.Wait()
	if err != nil {
		c.logf(0, "docker-compose pull failed: %s\n", err)
		return err
	}
	c.logf(1, "docker-compose pull took %s\n", time.Since(dockerPullStart))

	args := []string{"-f", c.dockerComposeFile, "up", "-d"}
	if verbose {
		args = append([]string{"--verbose"}, args...)
	}

	ctx, cancel := context.WithTimeout(ctx, c.dockerMaxWait)
	defer cancel()

	cmd, err = dockerCompose(ctx, verbose, args...)
	if err != nil {
		return err
	}

	cmd.WaitDelay = c.dockerMaxWait
	dockerStart := time.Now()
	err = cmd.Start()
	if err != nil {
		return err
	}
	c.dockerStarted = true

	c.logf(1, "Waiting for services to start...\n")
	err = cmd.Wait()
	if err != nil {
		c.logf(0, "docker-compose services failed to start: %s\n", err)
		return err
	}

	err = c.waitForPorts(ctx, c.dockerTCPPorts)
	if err != nil {
		c.logf(1, "docker-compose services took too long to start (%s)\n", c.dockerMaxWait)
		return err
	}
	dockerReady := time.Now()
	c.logf(1, "docker-compose services took %s to start\n", dockerReady.Sub(dockerStart))

	return nil
}

func (c *IDock) cleanup() {
	c.logf(1, "Cleaning up...\n")

	if !c.dockerStarted {
		return
	}

	if c.cleanupAttempts < 1 {
		c.logf(0, "Docker container left intact. To cleanup run:\n")
		c.logf(0, "docker-compose down --remove-orphans\n")
		return
	}

	args := []string{"-f", c.dockerComposeFile, "down", "--remove-orphans"}
	cmd, err := dockerCompose(context.Background(), c.verbosity > 1, args...)
	if err == nil {
		for i := 0; i < c.cleanupAttempts; i++ {
			err := cmd.Run()
			if err == nil {
				return
			}
			c.logf(1, "Failed to clean up docker-compose services on try %d: %s\n", i+1, err)
		}
	}

	fmt.Printf("Failed to clean up docker services. Please run `docker-compose down --remove-orphans` manually\n")
}

func (c *IDock) startProgram(ctx context.Context) error {
	start := time.Now()
	ctx, cancel := context.WithTimeout(ctx, c.programMaxWait)
	defer cancel()

	go c.program()

	err := c.waitForPorts(ctx, c.programTCPPorts)
	end := time.Now()
	if err != nil {
		c.logf(1, "program took too long to start: %s\n", end.Sub(start))
		return err
	}
	c.logf(1, "program startup took %s\n", end.Sub(start))

	return nil
}

func (c *IDock) isPortOpen(ctx context.Context, port int) error {
	var d net.Dialer

	address := fmt.Sprintf("%s:%d", c.localhost, port)

	conn, err := d.DialContext(ctx, "tcp", address)
	if err != nil {
		return errTimedOut
	}
	conn.Close()

	return nil
}

func (c *IDock) waitForPorts(ctx context.Context, ports []int) error {
	var wg sync.WaitGroup

	results := make(chan int, len(ports))
	for _, port := range ports {
		wg.Add(1)
		go func(ctx context.Context, port int) {
			for {
				sub, cancel := context.WithTimeout(ctx, c.tcpPortMaxWait)

				err := c.isPortOpen(sub, port)
				if err == nil {
					results <- port
					cancel()
					wg.Done()
					return
				}

				// timing out while checking is not fatal unless the
				// root context is done.
				if errors.Is(err, errTimedOut) && ctx.Err() == nil {
					cancel()
					// Give the service a chance to start.
					time.Sleep(c.tcpPortMaxWait)
					continue
				}

				results <- (-1 * port)
				cancel()
				wg.Done()
				return
			}
		}(ctx, port)
	}

	wg.Wait()
	close(results)

	succeeded := make([]int, 0, len(ports))
	failed := make([]int, 0, len(ports))

	for result := range results {
		if result < 0 {
			failed = append(failed, (-1 * result))
		} else {
			succeeded = append(succeeded, result)
		}
	}

	sort.Ints(succeeded)
	for _, port := range succeeded {
		c.logf(1, "Port %d started\n", port)
	}

	sort.Ints(failed)
	for _, port := range failed {
		c.logf(1, "Port %d failed to start\n", port)
	}
	if len(failed) > 0 {
		return errTimedOut
	}

	c.logf(1, "All services are ready.\n")
	return nil
}

// logf prints a message if the verbosity level is greater than or equal to the
// given level.
func (c *IDock) logf(level int, format string, a ...any) {
	if c.verbosity >= level {
		fmt.Printf(format, a...)
	}
}

func dockerCompose(ctx context.Context, useStdout bool, args ...string) (*exec.Cmd, error) {
	cmd := exec.CommandContext(ctx, "docker-compose", args...)
	if cmd == nil {
		return nil, errors.New("failed to create docker-compose command")
	}

	// Some systems don't have docker-compose installed, so try to use the docker-compose
	// binary from the docker image instead.
	if errors.Is(cmd.Err, exec.ErrNotFound) {
		args = append([]string{"compose"}, args...)
		cmd = exec.CommandContext(ctx, "docker", args...)
		if cmd == nil {
			return nil, errors.New("failed to create docker-compose command")
		}
	}

	if cmd.Err != nil {
		return nil, cmd.Err
	}

	cmd.Stderr = os.Stderr
	if useStdout {
		cmd.Stdout = os.Stdout
	}

	return cmd, nil
}
