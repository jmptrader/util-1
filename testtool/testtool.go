// Copyright 2013 Apcera Inc. All rights reserved.

package testtool

import (
	"flag"
	"io/ioutil"
	"math/rand"
	"os"
	"testing"

	"github.com/apcera/logray"
	"github.com/apcera/logray/unittest"
)

// Common interface that can be used to allow testing.B and testing.T objects
// to be passed to the same function.
type Logger interface {
	Error(args ...interface{})
	Errorf(format string, args ...interface{})
	Failed() bool
	Fatal(args ...interface{})
	Fatalf(format string, args ...interface{})
	Skip(args ...interface{})
	Skipf(format string, args ...interface{})
	Log(args ...interface{})
	Logf(format string, args ...interface{})
}

// Backtracer is an interface for if an error implements the Backtrace() function then that backtrace will be
// displayed using the TestExpectSuccess() functions. For an example see
// BackError in the apcera/cfg package.
type Backtracer interface {
	Backtrace() []string
}

// -----------------------------------------------------------------------
// Initialization, cleanup, and shutdown functions.
// -----------------------------------------------------------------------

// If this flag is set to true then output will be displayed live as it
// happens rather than being buffered and only displayed when tests fail.
var streamTestOutput bool

// For help in debugging the tests give a -debug on the command line when
// executing the tests and it will be set to true. This value is used
// internally to signal that longer output strings should be allowed but is
// exposed to allow callers to use as well.
var TestDebug bool

// If a -log or log is provided with an path to a directory then that path is
// available in this variable. This is a helper for tests that wish to log. An
// empty string indicates the path was not set. The value is set only to allow
// callers to make use of in their tests. There are no other side effects.
var TestLogFile = ""

func init() {
	if f := flag.Lookup("debug"); f == nil {
		flag.BoolVar(
			&TestDebug,
			"debug",
			false,
			"Enable longer failure description.")
	}
	if f := flag.Lookup("log"); f == nil {
		flag.StringVar(
			&TestLogFile,
			"log",
			"",
			"Specifies the log file for the test")
	}
	if f := flag.Lookup("live-output"); f == nil {
		flag.BoolVar(
			&streamTestOutput,
			"live-output",
			false,
			"Enable output to be streamed live rather than buffering.")
	}
}

// TestTool type allows for parallel tests.
type TestTool struct {
	*testing.T

	// Stores output from the logging system so it can be written only if
	// the test actually fails.
	LogBuffer *unittest.LogBuffer

	// This is a list of functions that will be run on test completion. Having
	// this allows us to clean up temporary directories or files after the
	// test is done which is a huge win.
	Finalizers []func()

	// Parameters contains test-specific caches of data.
	Parameters map[string]interface{}

	RandoTestString string
}

// Adds a function to be called once the test finishes.
func (tt *TestTool) AddTestFinalizer(f func()) {
	tt.Finalizers = append(tt.Finalizers, f)
}

// StartTest should be called at the start of a test to setup all the various state bits that
// are needed. All tests in this module should start by calling this
// function.
func StartTest(t *testing.T) *TestTool {
	tt := TestTool{
		Parameters:      make(map[string]interface{}),
		T:               t,
		RandoTestString: RandoTestString(10),
	}

	if !streamTestOutput {
		tt.LogBuffer = unittest.SetupBuffer()
	} else {
		logray.AddDefaultOutput("stdout://", logray.ALL)
	}

	return &tt
}

// Called as a defer to a test in order to clean up after a test run. All
// tests in this module should call this function as a defer right after
// calling StartTest()
func (tt *TestTool) FinishTest() {
	for i := len(tt.Finalizers) - 1; i >= 0; i-- {
		tt.Finalizers[i]()
	}

	tt.Finalizers = nil
	if tt.LogBuffer != nil {
		tt.LogBuffer.FinishTest(tt.T)
	}
}

// -----------------------------------------------------------------------
// Temporary file helpers.
// -----------------------------------------------------------------------

// Writes contents to a temporary file, sets up a Finalizer to remove
// the file once the test is complete, and then returns the newly
// created filename to the caller.
func (tt *TestTool) WriteTempFile(contents string) string {
	return tt.WriteTempFileMode(contents, os.FileMode(0644))
}

// Makes a temporary directory
func (tt *TestTool) TempDir() string {
	return tt.TempDirMode(os.FileMode(0755))
}

// Allocate a temporary file and ensure that it gets cleaned up when the
// test is completed.
func (tt *TestTool) TempFile() string {
	return tt.TempFileMode(os.FileMode(0644))
}

// Like WriteTempFile but sets the mode.
func (tt *TestTool) WriteTempFileMode(contents string, mode os.FileMode) string {
	f, err := ioutil.TempFile("", "golangunittest")
	if f == nil {
		Fatalf(tt.T, "ioutil.TempFile() return nil.")
	} else if err != nil {
		Fatalf(tt.T, "ioutil.TempFile() return an err: %s", err)
	} else if err := os.Chmod(f.Name(), mode); err != nil {
		Fatalf(tt.T, "os.Chmod() returned an error: %s", err)
	}
	defer f.Close()
	tt.AddTestFinalizer(func() {
		os.Remove(f.Name())
	})
	contentsBytes := []byte(contents)
	n, err := f.Write(contentsBytes)
	if err != nil {
		Fatalf(tt.T, "Error writing to %s: %s", f.Name(), err)
	} else if n != len(contentsBytes) {
		Fatalf(tt.T, "Short write to %s", f.Name())
	}
	return f.Name()
}

// Makes a temporary directory with the given mode.
func (tt *TestTool) TempDirMode(mode os.FileMode) string {
	f, err := ioutil.TempDir(RootTempDir(tt), "golangunittest")
	if f == "" {
		Fatalf(tt.T, "ioutil.TempFile() return an empty string.")
	} else if err != nil {
		Fatalf(tt.T, "ioutil.TempFile() return an err: %s", err)
	} else if err := os.Chmod(f, mode); err != nil {
		Fatalf(tt.T, "os.Chmod failure.")
	}

	tt.AddTestFinalizer(func() {
		os.RemoveAll(f)
	})

	return f
}

// Writes a temp file with the given mode.
func (tt *TestTool) TempFileMode(mode os.FileMode) string {
	f, err := ioutil.TempFile(RootTempDir(tt), "unittest")
	if err != nil {
		Fatalf(tt.T, "Error making temporary file: %s", err)
	} else if err := os.Chmod(f.Name(), mode); err != nil {
		Fatalf(tt.T, "os.Chmod failure.")
	}
	defer f.Close()
	name := f.Name()
	tt.Finalizers = append(tt.Finalizers, func() {
		os.RemoveAll(name)
	})
	return name
}

var letters = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")

func RandoTestString(n int) string {
	b := make([]rune, n)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}

	return string(b)
}
