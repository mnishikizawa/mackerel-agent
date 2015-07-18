package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"math"
	"os"
	"runtime"
	"strings"
	"testing"
)

func TestParseFlags(t *testing.T) {
	// prepare dummy config
	confFile, err := ioutil.TempFile("", "mackerel-config-test")

	if err != nil {
		t.Fatalf("Could not create temprary config file for test")
	}
	confFile.WriteString(`verbose=false
root="/hoge/fuga"
apikey="DUMMYAPIKEY"
diagnostic=false
`)
	confFile.Sync()
	confFile.Close()
	defer os.Remove(confFile.Name())

	os.Args = []string{"mackerel-agent", "-conf=" + confFile.Name(), "-role=My-Service:default,INVALID#SERVICE", "-verbose", "-diagnostic"}
	// Overrides Args from go test command
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.PanicOnError)

	mergedConfig, _ := resolveConfig()

	t.Logf("      apibase: %v", mergedConfig.Apibase)
	t.Logf("       apikey: %v", mergedConfig.Apikey)
	t.Logf("         root: %v", mergedConfig.Root)
	t.Logf("      pidfile: %v", mergedConfig.Pidfile)
	t.Logf("   diagnostic: %v", mergedConfig.Diagnostic)
	t.Logf("roleFullnames: %v", mergedConfig.Roles)
	t.Logf("      verbose: %v", mergedConfig.Verbose)

	if mergedConfig.Root != "/hoge/fuga" {
		t.Errorf("Root(confing from file) should be /hoge/fuga but: %v", mergedConfig.Root)
	}

	if len(mergedConfig.Roles) != 1 || mergedConfig.Roles[0] != "My-Service:default" {
		t.Error("Roles(config from command line option) should be parsed")
	}

	if mergedConfig.Verbose != true {
		t.Error("Verbose(overwritten by command line option) shoud be true")
	}

	if mergedConfig.Diagnostic != true {
		t.Error("Diagnostic(overwritten by command line option) shoud be true")
	}
}

func TestParseFlagsPrintVersion(t *testing.T) {
	os.Args = []string{"mackerel-agent", "-version"}
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.PanicOnError)

	config, otherOptions := resolveConfig()

	if config.Verbose != false {
		t.Error("with -version args, variables of config should have default values")
	}

	if otherOptions.printVersion == false {
		t.Error("with -version args, printVersion should be true")
	}
}

func TestParseFlagsRunOnce(t *testing.T) {
	os.Args = []string{"mackerel-agent", "-once"}
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.PanicOnError)

	config, otherOptions := resolveConfig()

	if config.Verbose != false {
		t.Error("with -version args, variables of config should have default values")
	}

	if otherOptions.runOnce == false {
		t.Error("with -once args, RunOnce should be true")
	}
}

func TestCreateAndRemovePidFile(t *testing.T) {
	file, err := ioutil.TempFile("", "")
	if err != nil {
		t.Errorf("failed to create tmpfile, %s", err)
	}
	fpath := file.Name()
	defer os.Remove(fpath)

	err = createPidFile(fpath)
	if err != nil {
		t.Errorf("pid file should be created but, %s", err)
	}

	if runtime.GOOS != "windows" {
		if err := createPidFile(fpath); err == nil || !strings.HasPrefix(err.Error(), "Pidfile found, try stopping another running mackerel-agent or delete") {
			t.Errorf("creating pid file should be failed when the running process exists, %s", err)
		}
	}

	removePidFile(fpath)
	if err := createPidFile(fpath); err != nil {
		t.Errorf("pid file should be created but, %s", err)
	}

	removePidFile(fpath)
	ioutil.WriteFile(fpath, []byte(fmt.Sprint(math.MaxInt32)), 0644)
	if err := createPidFile(fpath); err != nil {
		t.Errorf("old pid file should be ignored and new pid file should be created but, %s", err)
	}
}
