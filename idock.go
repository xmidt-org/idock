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
	"testing"
	"time"
)

const (
	// IDOCK_RUN_FLAG is the environment variable that controls whether or not
	// to run the integration tests.
	RUN_FLAG = "IDOCK_RUN"

	// IDOCK_VERBOSITY_FLAG is the environment variable that controls the
	// verbosity of the output.
	VERBOSITY_FLAG = "IDOCK_VERBOSITY"

	// DOCKER_MAX_WAIT_FLAG is the environment variable that controls how long
	// to wait for the docker-compose program to start.
	DOCKER_MAX_WAIT_FLAG = "IDOCK_DOCKER_MAX_WAIT"

	// PROGRAM_MAX_WAIT_FLAG is the environment variable that controls how long
	// to wait for the program to start.
	PROGRAM_MAX_WAIT_FLAG = "IDOCK_PROGRAM_MAX_WAIT"

	// CLEANUP_RETRIES_FLAG is the environment variable that controls how many
	// times to retry the cleanup process.
	CLEANUP_RETRIES_FLAG = "IDOCK_CLEANUP_RETRIES"
)

var (
	errTimedOut      = fmt.Errorf("timed out")
	errProgramExited = fmt.Errorf("program exited")
)

// IDock is the main struct for the idock package.
type IDock struct {
	tcpPortMaxWait    time.Duration
	dockerComposeFile string
	dockerTCPPorts    []int
	dockerMaxWait     time.Duration
	afterDocker       func(*IDock)
	program           func()
	programTCPPorts   []int
	programMaxWait    time.Duration
	afterProgram      func(*IDock) error
	cleanupRetries    int
	localhost         string
	verbosity         int
	noDockerCompose   bool
}

// Option is an option interface for the IDock struct.
type Option interface {
	apply(*IDock)
}

type optionFunc func(*IDock)

func (f optionFunc) apply(c *IDock) {
	f(c)
}

// New creates a new IDock struct with the given options.
func New(opts ...Option) *IDock {
	c := &IDock{
		tcpPortMaxWait: 2 * time.Second,
		afterDocker:    func(*IDock) {},
		program:        func() {},
		afterProgram: func(*IDock) error {
			return nil
		},
		programMaxWait: 2 * time.Second,
		localhost:      "localhost",
		cleanupRetries: 3,
	}

	opts = append(opts, []Option{
		verbosity(),
		cleanupRetries(),
		dockerMaxWait(),
		programMaxWait(),
	}...)

	for _, opt := range opts {
		opt.apply(c)
	}

	return c
}

// Verbosity gets the verbosity level.
func (c *IDock) Verbosity() int {
	return c.verbosity
}

// Run runs the integration tests.
func (c *IDock) Run(m *testing.M) {
	if os.Getenv(RUN_FLAG) == "" {
		return
	}

	code := c.run(m)

	c.Logf(1, "Exiting with code %d\n", code)

	os.Exit(code)
}

func (c *IDock) startDocker() error {
	if c.dockerComposeFile == "" {
		c.cleanupRetries = 0
		return nil
	}

	args := []string{"-f", c.dockerComposeFile, "up", "-d"}
	if c.verbosity > 1 {
		args = append([]string{"--verbose"}, args...)
	}

	cmd := exec.Command("docker-compose", args...)

	// Some systems don't have docker-compose installed, so try to use the docker-compose
	// binary from the docker image instead.
	if errors.Is(cmd.Err, exec.ErrNotFound) {
		args = append([]string{"compose"}, args...)
		cmd = exec.Command("docker", args...)
		c.noDockerCompose = true
	}

	if c.verbosity > 1 {
		cmd.Stderr = os.Stdout
		cmd.Stdout = os.Stdout
	}

	dockerStart := time.Now()
	err := cmd.Start()
	if err != nil {
		c.cleanupRetries = 0
		return err
	}

	c.Logf(1, "Waiting for services to start...\n")
	ctx, cancel := context.WithTimeout(context.Background(), c.dockerMaxWait)
	defer cancel()
	err = c.wait(ctx, nil)
	if err != nil {
		c.Logf(1, "docker-compose services took too long to start (%s)\n", c.dockerMaxWait)
		return err
	}
	dockerReady := time.Now()
	c.Logf(1, "docker-compose services took %s to start\n", dockerReady.Sub(dockerStart))

	return nil
}

func (c *IDock) run(m *testing.M) int {
	err := c.startDocker()
	if 0 <= c.cleanupRetries {
		defer c.cleanup()
	}
	if err != nil {
		c.Logf(0, "docker startup failed: %s\n", err)
		return -1
	}

	if c.afterDocker != nil {
		start := time.Now()
		c.afterDocker(c)
		end := time.Now()
		c.Logf(1, "customization after docker took %s\n", end.Sub(start))
	}

	start := time.Now()
	done := make(chan struct{})
	c.safelyWrap()
	ctx, cancel := context.WithTimeout(context.Background(), c.programMaxWait)
	defer cancel()
	err = c.wait(ctx, c.programTCPPorts)
	end := time.Now()
	c.Logf(1, "program startup took %s\n", end.Sub(start))

	if err != nil {
		if errors.Is(err, errProgramExited) {
			c.Logf(0, "program exited before services were ready\n")
		} else if errors.Is(err, errTimedOut) {
			c.Logf(0, "program took too long to start\n")
		} else {
			c.Logf(0, "program had some unknown error: %s\n", err)
		}
		return -1
	}

	if c.afterProgram != nil {
		start := time.Now()
		err = c.afterProgram(c)
		end := time.Now()
		c.Logf(1, "customization after program took %s\n", end.Sub(start))
		if err != nil {
			c.Logf(0, "customization after program started issued an error: %s\n", err)
			return -1
		}
	}

	c.Logf(2, "running the tests\n")
	return m.Run()
}

func (c *IDock) isPortOpen(ctx context.Context, port int) (bool, error) {
	var d net.Dialer

	address := fmt.Sprintf("%s:%d", c.localhost, port)

	conn, err := d.DialContext(ctx, "tcp", address)
	if err != nil {
		return false, err
	}
	conn.Close()

	return true, nil
}

func (c *IDock) wait(ctx context.Context, ports []int) error {
	var wg sync.WaitGroup

	results := make(chan int, len(ports))
	for _, port := range ports {
		wg.Add(1)
		go func(ctx context.Context, port int) {
			for {
				sub, cancel := context.WithTimeout(ctx, 10*time.Millisecond)
				got, err := c.isPortOpen(sub, port)
				cancel()

				if err == nil && got {
					results <- port
					break
				}
				if err != nil {
					results <- (-1 * port)
					break
				}
			}
			wg.Done()
		}(ctx, port)
	}

	wg.Wait()

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
		c.Logf(1, "Port %d started\n", port)
	}

	sort.Ints(failed)
	for _, port := range failed {
		c.Logf(1, "Port %d failed to start\n", port)
	}
	if len(failed) > 0 {
		return errTimedOut
	}

	c.Logf(1, "All services are ready.\n")
	return nil
}

// Logf prints a message if the verbosity level is greater than or equal to the
// given level.
func (c *IDock) Logf(level int, format string, a ...any) {
	if level <= c.verbosity {
		return
	}
	fmt.Printf(format, a...)
}

func (c *IDock) cleanup() {
	c.Logf(1, "Cleaning up...\n")

	var cmd *exec.Cmd
	if c.noDockerCompose {
		cmd = exec.Command("docker", "compose", "-f", c.dockerComposeFile, "down", "--remove-orphans")
	} else {
		cmd = exec.Command("docker-compose", "-f", c.dockerComposeFile, "down", "--remove-orphans")
	}

	if c.verbosity > 1 {
		cmd.Stderr = os.Stdout
		cmd.Stdout = os.Stdout
	}

	for i := 0; i < c.cleanupRetries; i++ {
		err := cmd.Run()
		if err == nil {
			return
		}
		c.Logf(1, "Failed to clean up docker-compose services on try %d: %s\n", i+1, err)
	}

	fmt.Printf("Failed to clean up docker services. Please run `docker-compose down --remove-orphans` manually\n")
}

func (c *IDock) safelyWrap() bool {
	ctx, cancel := context.WithTimeout(context.Background(), c.programMaxWait)
	defer cancel()

	failure := make(chan bool)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				fmt.Println("recovered from panic")
				fmt.Println(r)
				failure <- true
			}
		}()

		c.program()
	}()

	select {
	case <-failure:
		return success
	case <-ctx.Done():
		// The program started and didn't fail in the given time, so return true.
		return true
	}
}
