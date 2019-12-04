// Package apptest provides utilities for testing cli.App.
package apptest

import (
	"testing"

	"github.com/elves/elvish/cli"
	"github.com/elves/elvish/cli/term"
)

// Fixture is a test fixture.
type Fixture struct {
	App    cli.App
	TTY    cli.TTYCtrl
	width  int
	codeCh <-chan string
	errCh  <-chan error
}

// Setup sets up a test fixture. It contains an App whose ReadCode method has
// been started asynchronously.
func Setup(fns ...func(*cli.AppSpec, cli.TTYCtrl)) *Fixture {
	tty, ttyCtrl := cli.NewFakeTTY()
	spec := cli.AppSpec{TTY: tty}
	for _, fn := range fns {
		fn(&spec, ttyCtrl)
	}
	app := cli.NewApp(spec)
	codeCh, errCh := start(app)
	_, width := tty.Size()
	return &Fixture{app, ttyCtrl, width, codeCh, errCh}
}

// WithSpec takes a function that operates on *cli.AppSpec, and wraps it into a
// form suitable for passing to Setup.
func WithSpec(f func(*cli.AppSpec)) func(*cli.AppSpec, cli.TTYCtrl) {
	return func(spec *cli.AppSpec, _ cli.TTYCtrl) { f(spec) }
}

// WithTTY takes a function that operates on cli.TTYCtrl, and wraps it to a form
// suitable for passing to Setup.
func WithTTY(f func(cli.TTYCtrl)) func(*cli.AppSpec, cli.TTYCtrl) {
	return func(_ *cli.AppSpec, tty cli.TTYCtrl) { f(tty) }
}

func start(app cli.App) (<-chan string, <-chan error) {
	codeCh := make(chan string, 1)
	errCh := make(chan error, 1)
	go func() {
		code, err := app.ReadCode()
		codeCh <- code
		errCh <- err
		close(codeCh)
		close(errCh)
	}()
	return codeCh, errCh
}

// Wait waits for ReaCode to finish, and returns its return values.
func (f *Fixture) Wait() (string, error) {
	return <-f.codeCh, <-f.errCh
}

// Stop stops ReadCode and waits for it to finish. If ReadCode has already been
// stopped, it is a no-op.
func (f *Fixture) Stop() {
	f.App.CommitEOF()
	f.Wait()
}

// MakeBuffer is a helper for building a buffer. It is equivalent to
// term.NewBufferBuilder(width of terminal).MarkLines(args...).Buffer().
func (f *Fixture) MakeBuffer(args ...interface{}) *term.Buffer {
	return term.NewBufferBuilder(f.width).MarkLines(args...).Buffer()
}

// TestTTY is equivalent to f.TTY.TestBuffer(f.MakeBuffer(args...)).
func (f *Fixture) TestTTY(t *testing.T, args ...interface{}) {
	t.Helper()
	f.TTY.TestBuffer(t, f.MakeBuffer(args...))
}
