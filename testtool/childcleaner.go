package testtool

import (
	"fmt"
	"io/ioutil"
	"os"
	"strings"
)

const startupInterceptorToken = "sdf908s0dijflk23423"

// This function is used to intercept the process startup and check to see if
// if its a clean up process.
func init() {
	if len(os.Args) != 3 {
		return
	} else if os.Args[1] != startupInterceptorToken {
		return
	}

	// Do NOT remove anything unless its in the temporary directory.
	if !strings.HasPrefix(os.Args[2], os.TempDir()) {
		fmt.Fprintf(
			os.Stderr, "Will not run on %s, its not in %s",
			os.Args[2], os.TempDir())
		os.Exit(1)
	}

	// Wait for stdin to be closed, once that happens we nuke the directory
	// in the third argument.
	if _, err := ioutil.ReadAll(os.Stdin); err != nil {
		fmt.Fprintf(
			os.Stderr, "Error cleaning up directory %s: %s\n",
			os.Args[2], err)
	} else if err := os.RemoveAll(os.Args[2]); err != nil {
		fmt.Fprintf(
			os.Stderr, "Error cleaning up directory %s: %s\n",
			os.Args[2], err)
	}
	os.Exit(0)
}

// Creates a directory that will exist until the process running the tests
// exits.
func RootTempDir(t *TestTool) string {
	if rd, ok := t.Parameters["RootDir"].(string); ok && rd != "" {
		return rd
	}

	var rootDirectory string

	var err error

	mode := os.FileMode(0777)
	rootDirectory, err = ioutil.TempDir("", t.RandoTestString)
	if rootDirectory == "" {
		Fatalf(t, "ioutil.TempFile() return an empty string.")
	} else if err != nil {
		Fatalf(t, "ioutil.TempFile() return an err: %s", err)
	} else if err := os.Chmod(rootDirectory, mode); err != nil {
		Fatalf(t, "os.Chmod error: %s", err)
	}

	t.Parameters["RootDir"] = rootDirectory

	t.AddTestFinalizer(func() {
		os.RemoveAll(rootDirectory)
	})

	return rootDirectory
}
