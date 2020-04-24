// Package program provides the entry point to Elvish. Its subpackages
// correspond to subprograms of Elvish.
package program

// This package sets up the basic environment and calls the appropriate
// "subprogram", one of the daemon, the terminal interface, or the web
// interface.

import (
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"runtime/pprof"
	"strconv"

	"github.com/elves/elvish/pkg/util"
)

// Default port on which the web interface runs. The number is chosen because it
// resembles "elvi".
const defaultWebPort = 3171

// Flags keeps command-line flags.
type Flags struct {
	Log, LogPrefix, CPUProfile string

	Help, Version, BuildInfo, JSON bool

	CodeInArg, CompileOnly, NoRc bool

	Web  bool
	Port int

	Daemon bool
	Forked int

	Bin, DB, Sock string
}

func newFlagSet(stderr io.Writer, f *Flags) *flag.FlagSet {
	fs := flag.NewFlagSet("elvish", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() { usage(stderr, fs) }

	fs.StringVar(&f.Log, "log", "", "a file to write debug log to")
	fs.StringVar(&f.LogPrefix, "logprefix", "", "the prefix for the daemon log file")
	fs.StringVar(&f.CPUProfile, "cpuprofile", "", "write cpu profile to file")

	fs.BoolVar(&f.Help, "help", false, "show usage help and quit")
	fs.BoolVar(&f.Version, "version", false, "show version and quit")
	fs.BoolVar(&f.BuildInfo, "buildinfo", false, "show build info and quit")
	fs.BoolVar(&f.JSON, "json", false, "show output in JSON. Useful with -buildinfo.")

	fs.BoolVar(&f.CodeInArg, "c", false, "take first argument as code to execute")
	fs.BoolVar(&f.CompileOnly, "compileonly", false, "Parse/Compile but do not execute")
	fs.BoolVar(&f.NoRc, "norc", false, "run elvish without invoking rc.elv")

	fs.BoolVar(&f.Web, "web", false, "run backend of web interface")
	fs.IntVar(&f.Port, "port", defaultWebPort, "the port of the web backend")

	fs.BoolVar(&f.Daemon, "daemon", false, "run daemon instead of shell")

	fs.StringVar(&f.Bin, "bin", "", "path to the elvish binary")
	fs.StringVar(&f.DB, "db", "", "path to the database")
	fs.StringVar(&f.Sock, "sock", "", "path to the daemon socket")

	return fs
}

func usage(out io.Writer, f *flag.FlagSet) {
	fmt.Fprintln(out, "Usage: elvish [flags] [script]")
	fmt.Fprintln(out, "Supported flags:")
	f.PrintDefaults()
}

func Main(fds [3]*os.File, args []string) int {
	return run(fds, args,
		versionProgram{}, buildInfoProgram{},
		daemonProgram{}, webProgram{}, shellProgram{})
}

func run(fds [3]*os.File, args []string, programs ...Program) int {
	f := &Flags{}
	fs := newFlagSet(fds[2], f)
	err := fs.Parse(args[1:])
	if err != nil {
		// Error and usage messages are already shown.
		return 2
	}

	// Handle flags common to all subprograms.
	if f.CPUProfile != "" {
		f, err := os.Create(f.CPUProfile)
		if err != nil {
			log.Fatal(err)
		}
		pprof.StartCPUProfile(f)
		defer pprof.StopCPUProfile()
	}

	if f.Log != "" {
		err = util.SetOutputFile(f.Log)
	} else if f.LogPrefix != "" {
		err = util.SetOutputFile(f.LogPrefix + strconv.Itoa(os.Getpid()))
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
	}

	if f.Help {
		fs.SetOutput(fds[1])
		usage(fds[1], fs)
		return 0
	}

	for _, program := range programs {
		if program.ShouldRun(f) {
		}
	}

	p := findProgram(f, programs)
	if p == nil {
		fmt.Fprintln(fds[2], "program bug: no suitable subprogram")
		return 2
	}

	err = p.Run(fds, f, fs.Args())
	if err == nil {
		return 0
	}
	if msg := err.Error(); msg != "" {
		fmt.Fprintln(fds[2], msg)
	}
	switch err := err.(type) {
	case badUsageError:
		usage(fds[2], fs)
	case exitError:
		return err.exit
	}
	return 2
}

func findProgram(f *Flags, programs []Program) Program {
	for _, program := range programs {
		if program.ShouldRun(f) {
			return program
		}
	}
	return nil
}

// BadUsage returns an error that may be returned by Program.Main, which
// requests the main program to print out a message, the usage information and
// exit with 2.
func BadUsage(msg string) error { return badUsageError{msg} }

type badUsageError struct{ msg string }

func (e badUsageError) Error() string { return e.msg }

// Exit returns an error that may be returned by Program.Main, which requests the
// main program to exit with the given code. If the exit code is 0, it returns nil.
func Exit(exit int) error {
	if exit == 0 {
		return nil
	}
	return exitError{exit}
}

type exitError struct{ exit int }

func (e exitError) Error() string { return "" }

// Program represents a subprogram.
type Program interface {
	// ShouldRun returns whether the subprogram should run.
	ShouldRun(f *Flags) bool
	// Run runs the subprogram.
	Run(fds [3]*os.File, f *Flags, args []string) error
}
