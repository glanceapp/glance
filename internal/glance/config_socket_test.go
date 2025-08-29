package glance

import (
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSocketModeValidation(t *testing.T) {
	tests := []struct {
		name        string
		config      config
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid socket mode with socket path",
			config: func() config {
				c := config{Pages: []page{{Title: "Test", Columns: []struct {
					Size    string  `yaml:"size"`
					Widgets widgets `yaml:"widgets"`
				}{{Size: "full"}}}}}
				c.Server.SocketPath = "/tmp/test.sock"
				c.Server.SocketMode = "0666"
				return c
			}(),
			expectError: false,
		},
		{
			name: "socket mode without socket path should fail",
			config: func() config {
				c := config{Pages: []page{{Title: "Test"}}}
				c.Server.Host = "localhost"
				c.Server.Port = 8080
				c.Server.SocketMode = "0666"
				return c
			}(),
			expectError: true,
			errorMsg:    "socket-mode can only be specified when using socket-path",
		},
		{
			name: "invalid socket mode should fail",
			config: func() config {
				c := config{Pages: []page{{Title: "Test"}}}
				c.Server.SocketPath = "/tmp/test.sock"
				c.Server.SocketMode = "999"
				return c
			}(),
			expectError: true,
			errorMsg:    "invalid socket-mode '999': must be valid octal permissions (e.g., '0666', '666')",
		},
		{
			name: "non-numeric socket mode should fail",
			config: func() config {
				c := config{Pages: []page{{Title: "Test"}}}
				c.Server.SocketPath = "/tmp/test.sock"
				c.Server.SocketMode = "rwxr--r--"
				return c
			}(),
			expectError: true,
			errorMsg:    "invalid socket-mode 'rwxr--r--': must be valid octal permissions (e.g., '0666', '666')",
		},
		{
			name: "valid three-digit socket mode",
			config: func() config {
				c := config{Pages: []page{{Title: "Test", Columns: []struct {
					Size    string  `yaml:"size"`
					Widgets widgets `yaml:"widgets"`
				}{{Size: "full"}}}}}
				c.Server.SocketPath = "/tmp/test.sock"
				c.Server.SocketMode = "666"
				return c
			}(),
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := isConfigStateValid(&tt.config)
			if tt.expectError {
				if err == nil {
					t.Errorf("expected error but got none")
				} else if tt.errorMsg != "" && err.Error() != tt.errorMsg {
					t.Errorf("expected error message '%s' but got '%s'", tt.errorMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("expected no error but got: %v", err)
				}
			}
		})
	}
}

func TestSocketCreationWithMode(t *testing.T) {
	// Create a temporary directory for test sockets
	tempDir, err := os.MkdirTemp("", "glance_socket_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	socketPath := filepath.Join(tempDir, "test.sock")

	// Create a test application with socket configuration
	app := &application{
		CreatedAt: time.Now(),
		Config:    config{},
	}
	app.Config.Server.SocketPath = socketPath
	app.Config.Server.SocketMode = "0644"

	// Test socket creation
	start, stop := app.server()

	// Start the server in a goroutine
	done := make(chan error, 1)
	go func() {
		done <- start()
	}()

	// Give the server a moment to start
	time.Sleep(100 * time.Millisecond)

	// Check if socket was created
	if _, err := os.Stat(socketPath); os.IsNotExist(err) {
		t.Fatalf("Socket file was not created")
	}

	// Check socket permissions
	info, err := os.Stat(socketPath)
	if err != nil {
		t.Fatalf("Failed to stat socket file: %v", err)
	}

	expectedMode := fs.FileMode(0644)
	actualMode := info.Mode() & fs.ModePerm
	if actualMode != expectedMode {
		t.Errorf("Expected socket permissions %o, but got %o", expectedMode, actualMode)
	}

	// Stop the server
	if err := stop(); err != nil {
		t.Errorf("Failed to stop server: %v", err)
	}

	// Wait for server to stop
	select {
	case err := <-done:
		if err != nil && err.Error() != "http: Server closed" {
			t.Errorf("Server stopped with unexpected error: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Error("Server did not stop within timeout")
	}
}

func TestSocketCreationWithoutMode(t *testing.T) {
	// Create a temporary directory for test sockets
	tempDir, err := os.MkdirTemp("", "glance_socket_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	socketPath := filepath.Join(tempDir, "test.sock")

	// Create a test application with socket configuration but no mode
	app := &application{
		CreatedAt: time.Now(),
		Config:    config{},
	}
	app.Config.Server.SocketPath = socketPath
	// SocketMode is empty, should use default permissions

	// Test socket creation
	start, stop := app.server()

	// Start the server in a goroutine
	done := make(chan error, 1)
	go func() {
		done <- start()
	}()

	// Give the server a moment to start
	time.Sleep(100 * time.Millisecond)

	// Check if socket was created
	if _, err := os.Stat(socketPath); os.IsNotExist(err) {
		t.Fatalf("Socket file was not created")
	}

	// Stop the server
	if err := stop(); err != nil {
		t.Errorf("Failed to stop server: %v", err)
	}

	// Wait for server to stop
	select {
	case err := <-done:
		if err != nil && err.Error() != "http: Server closed" {
			t.Errorf("Server stopped with unexpected error: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Error("Server did not stop within timeout")
	}
}