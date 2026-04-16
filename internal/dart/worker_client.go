package dart

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
	"sync/atomic"
	"time"
)

// WorkerClient manages the Dart semantic analysis subprocess.
type WorkerClient struct {
	cmd     *exec.Cmd
	stdin   io.WriteCloser
	scanner *bufio.Scanner
	mu      sync.Mutex
	nextID  atomic.Int64
	verbose bool
}

// NewWorkerClient spawns the Dart worker at workerPath and returns a ready client.
func NewWorkerClient(workerPath string, verbose bool) (*WorkerClient, error) {
	dartExe, err := findDartExecutable()
	if err != nil {
		return nil, fmt.Errorf("dart not found in PATH: %w", err)
	}

	cmd := exec.Command(dartExe, "run", workerPath)
	cmd.Stderr = os.Stderr // forward Dart errors to our stderr

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("creating stdin pipe: %w", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("creating stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("starting dart worker: %w", err)
	}

	client := &WorkerClient{
		cmd:     cmd,
		stdin:   stdin,
		scanner: bufio.NewScanner(stdout),
		verbose: verbose,
	}

	// Verify worker is alive with a ping
	if err := client.ping(); err != nil {
		cmd.Process.Kill() //nolint:errcheck
		return nil, fmt.Errorf("dart worker ping failed: %w", err)
	}

	return client, nil
}

// AnalyzeProject asks the Dart worker to analyze the project and returns all symbols.
func (c *WorkerClient) AnalyzeProject(projectRoot string) ([]*Symbol, error) {
	id := int(c.nextID.Add(1))

	req := AnalyzeRequest{
		ID:     id,
		Method: "analyze_project",
		Params: AnalyzeParams{Root: projectRoot},
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if err := c.writeRequest(req); err != nil {
		return nil, err
	}

	resp, err := c.readResponse(60 * time.Second)
	if err != nil {
		return nil, err
	}
	if resp.Error != "" {
		return nil, fmt.Errorf("dart worker error: %s", resp.Error)
	}
	return resp.Symbols, nil
}

func (c *WorkerClient) ping() error {
	id := int(c.nextID.Add(1))
	req := AnalyzeRequest{ID: id, Method: "ping", Params: PingParams{}}

	c.mu.Lock()
	defer c.mu.Unlock()

	if err := c.writeRequest(req); err != nil {
		return err
	}
	_, err := c.readResponse(10 * time.Second)
	return err
}

func (c *WorkerClient) writeRequest(req any) error {
	data, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("marshaling request: %w", err)
	}
	data = append(data, '\n')
	_, err = c.stdin.Write(data)
	return err
}

func (c *WorkerClient) readResponse(timeout time.Duration) (*AnalyzeResponse, error) {
	done := make(chan struct{})
	var resp AnalyzeResponse
	var readErr error

	go func() {
		defer close(done)
		if c.scanner.Scan() {
			readErr = json.Unmarshal(c.scanner.Bytes(), &resp)
		} else {
			readErr = fmt.Errorf("dart worker stdout closed")
		}
	}()

	select {
	case <-done:
		return &resp, readErr
	case <-time.After(timeout):
		return nil, fmt.Errorf("dart worker timeout after %s", timeout)
	}
}

// Stop terminates the Dart worker subprocess.
func (c *WorkerClient) Stop() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.stdin != nil {
		c.stdin.Close()
	}
	if c.cmd.Process != nil {
		c.cmd.Process.Kill() //nolint:errcheck
	}
}

// findDartExecutable returns the path to the dart executable.
func findDartExecutable() (string, error) {
	if path, err := exec.LookPath("dart"); err == nil {
		return path, nil
	}
	// Flutter SDK puts dart in flutter/bin/cache/dart-sdk/bin/
	if flutterPath, err := exec.LookPath("flutter"); err == nil {
		// Try sibling dart
		dir := flutterPath[:len(flutterPath)-len("flutter")]
		dartPath := dir + "dart"
		if _, err := os.Stat(dartPath); err == nil {
			return dartPath, nil
		}
	}
	return "", fmt.Errorf("not found")
}
