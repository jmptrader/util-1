// Copyright 2013 Apcera Inc. All rights reserved.

package testtool

import (
	"fmt"
	"os"
	"path"
	"reflect"
	"runtime"
	"strings"
	"time"
)

// Call this to require that your test is run as root. NOTICE: this does not
// cause the test to FAIL. This seems like the most sane thing to do based on
// the shortcomings of Go's test utilities.
//
// As an added feature this function will append all skipped test names into
// the file name specified in the environment variable:
//   $SKIPPED_ROOT_TESTS_FILE
func TestRequiresRoot(l Logger) {
	getTestName := func() string {
		// Maximum function depth. This shouldn't be called when the stack is
		// 1024 calls deep (its typically called at the top of the Test).
		pc := make([]uintptr, 1024)
		callers := runtime.Callers(2, pc)
		testname := ""
		for i := 0; i < callers; i++ {
			if f := runtime.FuncForPC(pc[i]); f != nil {
				// Function names have the following formats:
				//   runtime.goexit
				//   testing.tRunner
				//   github.com/util/testtool.TestRequiresRoot
				// To find the real function name we split on . and take the
				// last element.
				names := strings.Split(f.Name(), ".")
				if strings.HasPrefix(names[len(names)-1], "Test") {
					testname = names[len(names)-1]
				}
			}
		}
		if testname == "" {
			Fatalf(l, "Can't figure out the test name.")
		}
		return testname
	}

	if os.Getuid() != 0 {
		// We support the ability to set an environment variables where the
		// names of all skipped tests will be logged. This is used to ensure
		// that they can be run with sudo later.
		fn := os.Getenv("SKIPPED_ROOT_TESTS_FILE")
		if fn != "" {
			// Get the test name. We do this using the runtime package. The
			// first function named Test* we assume is the outer test function
			// which is in turn the test name.
			flags := os.O_WRONLY | os.O_APPEND | os.O_CREATE
			f, err := os.OpenFile(fn, flags, os.FileMode(0644))
			TestExpectSuccess(l, err)
			defer f.Close()
			_, err = f.WriteString(getTestName() + "\n")
			TestExpectSuccess(l, err)
		}

		l.Skipf("This test must be run as root. Skipping.")
	}
}

// -----------------------------------------------------------------------
// Fatalf wrapper.
// -----------------------------------------------------------------------

// This function wraps Fatalf in order to provide a functional stack trace
// on failures rather than just a line number of the failing check. This
// helps if you have a test that fails in a loop since it will show
// the path to get there as well as the error directly.
func Fatalf(l Logger, f string, args ...interface{}) {
	lines := make([]string, 0, 100)
	msg := fmt.Sprintf(f, args...)
	lines = append(lines, msg)

	// Get the directory of testtool in order to ensure that we don't show
	// it in the stack traces (it can be spammy).
	_, myfile, _, _ := runtime.Caller(0)
	mydir := path.Dir(myfile)

	// Generate the Stack of callers:
	for i := 0; true; i++ {
		_, file, line, ok := runtime.Caller(i)
		if ok == false {
			break
		}
		// Don't print the stack line if its within testtool since its
		// annoying to see the testtool internals.
		if path.Dir(file) == mydir {
			continue
		}
		msg := fmt.Sprintf("%d - %s:%d", i, file, line)
		lines = append(lines, msg)
	}
	l.Fatalf("%s", strings.Join(lines, "\n"))
}

func Fatal(t Logger, args ...interface{}) {
	Fatalf(t, "%s", fmt.Sprint(args...))
}

// -----------------------------------------------------------------------
// Simple Timeout functions
// -----------------------------------------------------------------------

// runs the given function until 'timeout' has passed, sleeping 'sleep'
// duration in between runs. If the function returns true this exits,
// otherwise after timeout this will fail the test.
func Timeout(l Logger, timeout, sleep time.Duration, f func() bool) {
	end := time.Now().Add(timeout)
	for time.Now().Before(end) {
		if f() == true {
			return
		}
		time.Sleep(sleep)
	}
	Fatalf(l, "testtool: Timeout after %v", timeout)
}

// -----------------------------------------------------------------------
// Error object handling functions.
// -----------------------------------------------------------------------

// Fatal's the test if err is nil.
func TestExpectError(l Logger, err error, msg ...string) {
	reason := ""
	if len(msg) > 0 {
		reason = ": " + strings.Join(msg, "")
	}
	if err == nil {
		Fatalf(l, "Expected an error, got nil%s", reason)
	}
}

// isRealError detects if a nil of a type stored as a concrete type,
// rather than an error interface, was passed in.
func isRealError(err error) bool {
	if err == nil {
		return false
	}
	v := reflect.ValueOf(err)
	if !v.CanInterface() {
		return true
	}
	if v.IsNil() {
		return false
	}
	return true
}

// Fatal's the test if err is not nil and fails the test and output the reason
// for the failure as the err argument the same as Fatalf. If err implements the
// BackTracer interface a backtrace will also be displayed.
func TestExpectSuccess(l Logger, err error, msg ...string) {
	reason := ""
	if len(msg) > 0 {
		reason = ": " + strings.Join(msg, "")
	}
	if err != nil && isRealError(err) {
		lines := make([]string, 0, 50)
		lines = append(lines, fmt.Sprintf("Unexpected error: %s", err))
		if be, ok := err.(Backtracer); ok {
			for _, line := range be.Backtrace() {
				lines = append(lines, fmt.Sprintf(" * %s", line))
			}
		}
		Fatalf(l, "%s%s", strings.Join(lines, "\n"), reason)
	}
}

func TestExpectZeroLength(l Logger, size int) {
	if size != 0 {
		Fatalf(l, "Zero length expected")
	}
}

func TestExpectNonZeroLength(l Logger, size int) {
	if size == 0 {
		Fatalf(l, "Zero length found")
	}
}

// Will verify that a panic is called with the expected msg.
func TestExpectPanic(l Logger, f func(), msg string) {
	defer func(msg string) {
		if m := recover(); m != nil {
			TestEqual(l, msg, m)
		}
	}(msg)
	f()
	Fatalf(l, "Expected a panic with message '%s'\n", msg)
}
