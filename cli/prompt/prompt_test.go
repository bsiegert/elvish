package prompt

import (
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/elves/elvish/styled"
	"github.com/elves/elvish/util"
)

func TestPrompt_DefaultCompute(t *testing.T) {
	prompt := New(Config{})

	prompt.Trigger(false)
	testUpdate(t, prompt, styled.Plain("???> "))
}

func TestPrompt_ShowsComputedPrompt(t *testing.T) {
	prompt := New(Config{
		Compute: func() styled.Text { return styled.Plain(">>> ") }})

	prompt.Trigger(false)
	testUpdate(t, prompt, styled.Plain(">>> "))
}

func TestPrompt_StalePrompt(t *testing.T) {
	compute, unblock := blockedAutoIncPrompt()
	prompt := New(Config{
		Compute: compute,
		StaleThreshold: func() time.Duration {
			return 10 * time.Millisecond
		},
	})

	prompt.Trigger(true)
	// The compute function is blocked, so a stale version of the intial
	// "unknown" prompt will be shown.
	testUpdate(t, prompt, styled.MakeText("???> ", "inverse"))

	// The compute function will now return.
	unblock()
	// The returned prompt will now be used.
	testUpdate(t, prompt, styled.Plain("1> "))

	// Force a refresh.
	prompt.Trigger(true)
	// The compute function will now be blocked again, so after a while a stale
	// version of the previous prompt will be shown.
	testUpdate(t, prompt, styled.MakeText("1> ", "inverse"))

	// Unblock the compute function.
	unblock()
	// The new prompt will now be shown.
	testUpdate(t, prompt, styled.Plain("2> "))

	// Force a refresh.
	prompt.Trigger(true)
	// Make sure that the compute function is run and stuck.
	testUpdate(t, prompt, styled.MakeText("2> ", "inverse"))
	// Queue another two refreshes before the compute function can return.
	prompt.Trigger(true)
	prompt.Trigger(true)
	unblock()
	// Now the new prompt should be marked stale immediately.
	testUpdate(t, prompt, styled.MakeText("3> ", "inverse"))
	unblock()
	// However, the the two refreshes we requested early only trigger one
	// re-computation, because they are requested while the compute function is
	// stuck, so they can be safely merged.
	testUpdate(t, prompt, styled.Plain("4> "))
}

func TestPrompt_Eagerness0(t *testing.T) {
	prompt := New(Config{
		Compute:   autoIncPrompt(),
		Eagerness: func() int { return 0 },
	})

	// A forced refresh is always respected.
	prompt.Trigger(true)
	testUpdate(t, prompt, styled.Plain("1> "))

	// A unforced refresh is not respected.
	prompt.Trigger(false)
	testNoUpdate(t, prompt)

	// No update even if pwd has changed.
	_, cleanup := util.InTestDir()
	defer cleanup()
	prompt.Trigger(false)
	testNoUpdate(t, prompt)

	// Only force updates are respected.
	prompt.Trigger(true)
	testUpdate(t, prompt, styled.Plain("2> "))
}

func TestPrompt_Eagerness5(t *testing.T) {
	prompt := New(Config{
		Compute:   autoIncPrompt(),
		Eagerness: func() int { return 5 },
	})

	// The initial trigger is respected because there was no previous pwd.
	prompt.Trigger(false)
	testUpdate(t, prompt, styled.Plain("1> "))

	// No update because the pwd has not changed.
	prompt.Trigger(false)
	testNoUpdate(t, prompt)

	// Update because the pwd has changed.
	_, cleanup := util.InTestDir()
	defer cleanup()
	prompt.Trigger(false)
	testUpdate(t, prompt, styled.Plain("2> "))
}

func TestPrompt_Eagerness10(t *testing.T) {
	prompt := New(Config{
		Compute:   autoIncPrompt(),
		Eagerness: func() int { return 10 },
	})

	// The initial trigger is respected.
	prompt.Trigger(false)
	testUpdate(t, prompt, styled.Plain("1> "))

	// Subsequent triggers, force or not, are also respected.
	prompt.Trigger(false)
	testUpdate(t, prompt, styled.Plain("2> "))
	prompt.Trigger(true)
	testUpdate(t, prompt, styled.Plain("3> "))
	prompt.Trigger(false)
	testUpdate(t, prompt, styled.Plain("4> "))
}

func blockedAutoIncPrompt() (func() styled.Text, func()) {
	unblockChan := make(chan struct{})
	i := 0
	compute := func() styled.Text {
		<-unblockChan
		i++
		return styled.Plain(fmt.Sprintf("%d> ", i))
	}
	unblock := func() {
		unblockChan <- struct{}{}
	}
	return compute, unblock
}

func autoIncPrompt() func() styled.Text {
	i := 0
	return func() styled.Text {
		i++
		return styled.Plain(fmt.Sprintf("%d> ", i))
	}
}

func testUpdate(t *testing.T, p *Prompt, wantUpdate styled.Text) {
	t.Helper()
	select {
	case update := <-p.LateUpdates():
		if !reflect.DeepEqual(update, wantUpdate) {
			t.Errorf("got late update %v, want %v", update, wantUpdate)
		}
	case <-time.After(time.Second):
		t.Errorf("no late update after 1 second")
	}
	current := p.Get()
	if !reflect.DeepEqual(current, wantUpdate) {
		t.Errorf("got current %v, want %v", current, wantUpdate)
	}
}

func testNoUpdate(t *testing.T, p *Prompt) {
	t.Helper()
	select {
	case update := <-p.LateUpdates():
		t.Errorf("unexpected update %v", update)
	case <-time.After(10 * time.Millisecond):
		// OK
	}
}
