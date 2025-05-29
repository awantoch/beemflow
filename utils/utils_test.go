package utils

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ============================================================================
// LOGGER TESTS
// ============================================================================

func TestLoggerBasicFunctions(t *testing.T) {
	// Test basic logging functions
	User("test user message")
	Info("test info message")
	Warn("test warn message")
	Error("test error message")
	Debug("test debug message")

	// Test formatted error
	Errorf("test error with format: %s", "formatted")
}

func TestLoggerModes(t *testing.T) {
	// Test getting mode (avoid SetMode due to potential deadlock)
	mode := getMode()
	if mode == "" {
		t.Error("Expected non-empty mode")
	}

	// Test that mode is one of expected values
	validModes := map[string]bool{
		"production": true,
		"debug":      true,
		"info":       true,
		"warn":       true,
	}

	if !validModes[mode] {
		t.Logf("Current mode: %s (may be valid but not in our test list)", mode)
	}
}

func TestLoggerOutputs(t *testing.T) {
	// Test setting user output
	var userBuf bytes.Buffer
	SetUserOutput(&userBuf)
	User("test user output")
	if !strings.Contains(userBuf.String(), "test user output") {
		t.Error("User output not captured correctly")
	}

	// Test setting internal output
	var internalBuf bytes.Buffer
	SetInternalOutput(&internalBuf)
	Info("test internal output")
	if !strings.Contains(internalBuf.String(), "test internal output") {
		t.Error("Internal output not captured correctly")
	}

	// Reset to default outputs
	SetUserOutput(os.Stdout)
	SetInternalOutput(os.Stderr)
}

func TestLoggerWriter(t *testing.T) {
	// Test the LoggerWriter type
	var buf bytes.Buffer
	writer := &LoggerWriter{
		Fn: func(format string, v ...any) {
			buf.WriteString(fmt.Sprintf(format, v...))
		},
		Prefix: "[TEST] ",
	}

	n, err := writer.Write([]byte("test write message"))
	if err != nil {
		t.Errorf("LoggerWriter.Write failed: %v", err)
	}
	if n == 0 {
		t.Error("LoggerWriter.Write returned 0 bytes written")
	}

	if !strings.Contains(buf.String(), "[TEST] test write message") {
		t.Errorf("Expected '[TEST] test write message' in output, got: %s", buf.String())
	}

	// Test LoggerWriter with no prefix
	var buf2 bytes.Buffer
	writer2 := &LoggerWriter{
		Fn: func(format string, v ...any) {
			buf2.WriteString(fmt.Sprintf(format, v...))
		},
		Prefix: "",
	}

	writer2.Write([]byte("no prefix message"))
	if !strings.Contains(buf2.String(), "no prefix message") {
		t.Errorf("Expected 'no prefix message' in output, got: %s", buf2.String())
	}

	// Test LoggerWriter with multiline input
	var buf3 bytes.Buffer
	writer3 := &LoggerWriter{
		Fn: func(format string, v ...any) {
			buf3.WriteString(fmt.Sprintf(format, v...) + "\n")
		},
		Prefix: "",
	}

	writer3.Write([]byte("line1\nline2\nline3"))
	output := buf3.String()
	if !strings.Contains(output, "line1") || !strings.Contains(output, "line2") || !strings.Contains(output, "line3") {
		t.Errorf("Expected all lines in output, got: %s", output)
	}
}

func TestLoggerContext(t *testing.T) {
	// Test context-based logging
	ctx := context.Background()

	// Test WithRequestID
	ctxWithID := WithRequestID(ctx, "test-request-id")
	if ctxWithID == nil {
		t.Error("WithRequestID returned nil context")
	}

	// Test RequestIDFromContext
	requestID, ok := RequestIDFromContext(ctxWithID)
	if !ok {
		t.Error("RequestIDFromContext should return true for context with request ID")
	}
	if requestID != "test-request-id" {
		t.Errorf("Expected request ID 'test-request-id', got %s", requestID)
	}

	// Test context logging functions
	InfoCtx(ctxWithID, "test info with context")
	WarnCtx(ctxWithID, "test warn with context")
	ErrorCtx(ctxWithID, "test error with context")
	DebugCtx(ctxWithID, "test debug with context")

	// Test with context without request ID
	emptyRequestID, ok := RequestIDFromContext(ctx)
	if ok {
		t.Error("RequestIDFromContext should return false for context without request ID")
	}
	if emptyRequestID != "" {
		t.Errorf("Expected empty request ID, got %s", emptyRequestID)
	}

	// Test context logging with additional fields
	InfoCtx(ctxWithID, "test with fields", "key1", "value1", "key2", "value2")
	WarnCtx(ctx, "test without request ID", "field", "value")
}

func TestLoggerInitialization(t *testing.T) {
	// Test that loggers are initialized
	// This mainly tests that init() and initLoggers() don't panic
	initLoggers("debug")

	// Test that we can log after initialization
	Info("test after init")
}

func TestLoggerEdgeCases(t *testing.T) {
	// Test edge cases for logger functions

	// Test SetUserOutput with nil
	SetUserOutput(nil)
	User("test after nil user output")

	// Test SetInternalOutput with nil
	SetInternalOutput(nil)
	Info("test after nil internal output")

	// Reset to defaults
	SetUserOutput(os.Stdout)
	SetInternalOutput(os.Stderr)

	// Test logging when logger might be nil
	originalInternalLogger := internalLogger
	internalLogger = nil

	Info("test with nil internal logger")
	Warn("test warn with nil logger")
	Error("test error with nil logger")
	Debug("test debug with nil logger")

	// Test Errorf with nil logger
	err := Errorf("test error with nil logger: %s", "formatted")
	if err == nil {
		t.Error("Errorf should return error even with nil logger")
	}

	// Restore logger
	internalLogger = originalInternalLogger
}

func TestErrorf(t *testing.T) {
	// Test Errorf function specifically
	err := Errorf("test error: %s", "formatted")
	if err == nil {
		t.Error("Errorf should return an error")
	}

	expectedMsg := "test error: formatted"
	if err.Error() != expectedMsg {
		t.Errorf("Expected error message '%s', got '%s'", expectedMsg, err.Error())
	}
}

// ============================================================================
// TEST UTILITY TESTS
// ============================================================================

func TestCleanupDir(t *testing.T) {
	// Test CleanupDir function directly

	// Create a temporary directory
	tempDir, err := os.MkdirTemp("", "test-cleanup-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}

	// Create some files in the directory
	testFile1 := filepath.Join(tempDir, "file1.txt")
	testFile2 := filepath.Join(tempDir, "file2.txt")
	subDir := filepath.Join(tempDir, "subdir")

	if err := os.WriteFile(testFile1, []byte("content1"), 0644); err != nil {
		t.Fatalf("Failed to create test file 1: %v", err)
	}
	if err := os.WriteFile(testFile2, []byte("content2"), 0644); err != nil {
		t.Fatalf("Failed to create test file 2: %v", err)
	}
	if err := os.Mkdir(subDir, 0755); err != nil {
		t.Fatalf("Failed to create subdirectory: %v", err)
	}

	// Verify directory and files exist
	if _, err := os.Stat(tempDir); os.IsNotExist(err) {
		t.Fatalf("Temp directory does not exist before cleanup")
	}

	// Clean up the directory
	CleanupDir(tempDir)

	// Verify directory is removed
	if _, err := os.Stat(tempDir); !os.IsNotExist(err) {
		t.Errorf("Directory %s was not cleaned up properly", tempDir)
	}
}

func TestCleanupDirNonExistent(t *testing.T) {
	// Test CleanupDir with non-existent directory (should not panic)
	CleanupDir("/path/that/does/not/exist")

	// If we get here without panicking, the test passes
}

func TestCleanupDirPermissionError(t *testing.T) {
	// Create a directory and remove write permissions to test error handling
	tempDir, err := os.MkdirTemp("", "test-cleanup-perm-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}

	// Create a file in the directory
	testFile := filepath.Join(tempDir, "file.txt")
	if err := os.WriteFile(testFile, []byte("content"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Remove write permissions from the directory (on Unix systems)
	if err := os.Chmod(tempDir, 0444); err != nil {
		t.Fatalf("Failed to change directory permissions: %v", err)
	}

	// Try to clean up (this might fail due to permissions, but shouldn't panic)
	CleanupDir(tempDir)

	// Restore permissions and clean up manually
	os.Chmod(tempDir, 0755)
	os.RemoveAll(tempDir)
}

// TestWithCleanDirAndDirs tests the test utility functions indirectly
// by testing their core functionality through CleanupDir
func TestTestUtilityFunctions(t *testing.T) {
	// Test that WithCleanDir and WithCleanDirs exist and can be called
	// We can't easily test them directly without a testing.M, but we can
	// verify they exist and don't panic when called with nil

	// These functions are designed to be used in TestMain, so we just
	// verify they exist and can handle edge cases

	// Test that the functions exist (this will fail to compile if they don't)
	_ = WithCleanDir
	_ = WithCleanDirs

	// Test CleanupDir with multiple scenarios
	tempDir1, err := os.MkdirTemp("", "test-util-1-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir 1: %v", err)
	}

	tempDir2, err := os.MkdirTemp("", "test-util-2-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir 2: %v", err)
	}

	// Create files in both directories
	if err := os.WriteFile(filepath.Join(tempDir1, "file.txt"), []byte("content"), 0644); err != nil {
		t.Fatalf("Failed to create file in dir 1: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tempDir2, "file.txt"), []byte("content"), 0644); err != nil {
		t.Fatalf("Failed to create file in dir 2: %v", err)
	}

	// Clean up both directories
	CleanupDir(tempDir1)
	CleanupDir(tempDir2)

	// Verify both are cleaned up
	if _, err := os.Stat(tempDir1); !os.IsNotExist(err) {
		t.Errorf("Directory 1 was not cleaned up")
	}
	if _, err := os.Stat(tempDir2); !os.IsNotExist(err) {
		t.Errorf("Directory 2 was not cleaned up")
	}
}

func TestSetMode(t *testing.T) {
	// Save original mode
	originalMode := getMode()
	defer func() {
		// Restore original mode
		SetMode(originalMode)
	}()

	// Test setting various modes
	testModes := []string{"debug", "info", "warn", "production"}

	for _, mode := range testModes {
		SetMode(mode)
		currentMode := getMode()
		if currentMode != mode {
			t.Errorf("Expected mode %s, got %s", mode, currentMode)
		}
	}

	// Test that setting mode re-initializes loggers (no panic)
	SetMode("debug")
	Debug("test debug message after setting debug mode")

	SetMode("production")
	Info("test info message after setting production mode")
}

func TestWithCleanDir(t *testing.T) {
	// Test that WithCleanDir function exists and can be referenced
	// These functions are designed to be used in TestMain, so we just
	// verify they exist and test their core functionality through CleanupDir

	_ = WithCleanDir // Verify function exists

	// Test the core cleanup functionality that WithCleanDir uses
	tempDir, err := os.MkdirTemp("", "test-with-clean-dir-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}

	// Create a test file in the directory
	testFile := filepath.Join(tempDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("test content"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Verify directory exists
	if _, err := os.Stat(tempDir); os.IsNotExist(err) {
		t.Fatalf("Temp directory should exist before cleanup")
	}

	// Test the cleanup functionality directly
	CleanupDir(tempDir)

	// Verify directory is cleaned up
	if _, err := os.Stat(tempDir); !os.IsNotExist(err) {
		t.Errorf("Directory should be cleaned up after CleanupDir")
	}
}

func TestWithCleanDirs(t *testing.T) {
	// Test that WithCleanDirs function exists and can be referenced
	_ = WithCleanDirs // Verify function exists

	// Test the core cleanup functionality for multiple directories
	tempDir1, err := os.MkdirTemp("", "test-with-clean-dirs-1-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir 1: %v", err)
	}

	tempDir2, err := os.MkdirTemp("", "test-with-clean-dirs-2-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir 2: %v", err)
	}

	// Create test files in both directories
	if err := os.WriteFile(filepath.Join(tempDir1, "test1.txt"), []byte("content1"), 0644); err != nil {
		t.Fatalf("Failed to create test file 1: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tempDir2, "test2.txt"), []byte("content2"), 0644); err != nil {
		t.Fatalf("Failed to create test file 2: %v", err)
	}

	// Test cleanup of multiple directories
	CleanupDir(tempDir1)
	CleanupDir(tempDir2)

	// Verify both directories are cleaned up
	if _, err := os.Stat(tempDir1); !os.IsNotExist(err) {
		t.Errorf("Directory 1 should be cleaned up")
	}
	if _, err := os.Stat(tempDir2); !os.IsNotExist(err) {
		t.Errorf("Directory 2 should be cleaned up")
	}
}

func TestCleanupTempDir(t *testing.T) {
	// Test successful cleanup
	tempDir, err := os.MkdirTemp("", "test-cleanup-temp-dir-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}

	// Create a test file
	testFile := filepath.Join(tempDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("test content"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Test CleanupTempDir
	CleanupTempDir(tempDir)

	// Verify directory is removed
	if _, err := os.Stat(tempDir); !os.IsNotExist(err) {
		t.Errorf("Directory should be removed after CleanupTempDir")
	}

	// Test with non-existent directory (should not panic)
	CleanupTempDir("/path/that/does/not/exist")
}

func TestCleanupTempDirT(t *testing.T) {
	// Test successful cleanup
	tempDir, err := os.MkdirTemp("", "test-cleanup-temp-dir-t-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}

	// Create a test file
	testFile := filepath.Join(tempDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("test content"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Test CleanupTempDirT
	CleanupTempDirT(t, tempDir)

	// Verify directory is removed
	if _, err := os.Stat(tempDir); !os.IsNotExist(err) {
		t.Errorf("Directory should be removed after CleanupTempDirT")
	}

	// Test with non-existent directory (should not fail test)
	CleanupTempDirT(t, "/path/that/does/not/exist")
}

func TestWithCleanup(t *testing.T) {
	// Test the generic WithCleanup function
	var setupCalled, testCalled bool
	var tempDir string

	setup := func() (string, string) {
		setupCalled = true
		var err error
		tempDir, err = os.MkdirTemp("", "test-with-cleanup-*")
		if err != nil {
			t.Fatalf("Failed to create temp dir in setup: %v", err)
		}

		// Create a test file
		testFile := filepath.Join(tempDir, "resource.txt")
		if err := os.WriteFile(testFile, []byte("resource content"), 0644); err != nil {
			t.Fatalf("Failed to create resource file: %v", err)
		}

		return "test-resource", tempDir
	}

	test := func(resource string) {
		testCalled = true
		if resource != "test-resource" {
			t.Errorf("Expected resource 'test-resource', got %s", resource)
		}

		// Verify the directory and file exist during the test
		if _, err := os.Stat(tempDir); os.IsNotExist(err) {
			t.Errorf("Temp directory should exist during test")
		}

		resourceFile := filepath.Join(tempDir, "resource.txt")
		if _, err := os.Stat(resourceFile); os.IsNotExist(err) {
			t.Errorf("Resource file should exist during test")
		}
	}

	// Run WithCleanup
	WithCleanup(t, setup, test)

	// Verify both functions were called
	if !setupCalled {
		t.Error("Setup function should have been called")
	}
	if !testCalled {
		t.Error("Test function should have been called")
	}

	// Verify cleanup happened
	if _, err := os.Stat(tempDir); !os.IsNotExist(err) {
		t.Errorf("Directory should be cleaned up after WithCleanup")
	}
}
