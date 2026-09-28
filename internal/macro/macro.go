// Package macro runs the step sequences saved as config.Macro, for both the
// TUI and `jenklod-batman run`.
package macro

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"
	"time"

	"github.com/krank56/jenklod-batman/internal/config"
	"github.com/krank56/jenklod-batman/internal/jenkins"
)

// QueueTimeout bounds how long a build step waits for its build to leave
// the queue and get a number.
const QueueTimeout = 10 * time.Minute

// Event is a progress report: Step is 1-based.
type Event struct {
	Step, Total int
	Text        string
}

// Runner executes macros against one controller.
type Runner struct {
	Client *jenkins.Client
	// Poll is the delay between checks while waiting (default 5s).
	Poll time.Duration
	// Report receives progress; it may be nil.
	Report func(Event)
}

// StepError says which step stopped the macro.
type StepError struct {
	Step int
	Desc string
	Err  error
}

func (e *StepError) Error() string { return fmt.Sprintf("step %d (%s): %v", e.Step, e.Desc, e.Err) }
func (e *StepError) Unwrap() error { return e.Err }

// CheckOverrides rejects KEY=VALUE overrides that no build step uses, so a
// typo fails before anything is triggered.
func CheckOverrides(m config.Macro, overrides map[string]string) error {
	for _, k := range slices.Sorted(maps.Keys(overrides)) {
		used := false
		for _, s := range m.Steps {
			if _, ok := s.Params[k]; ok && s.Kind == config.StepBuild && s.WithParams {
				used = true
			}
		}
		if !used {
			return fmt.Errorf("%s is not a parameter of any build step in %q", k, m.Name)
		}
	}
	return nil
}

// Run executes the steps in order and stops at the first failure. The
// overrides replace saved parameter values in every build step that has
// them. Cancelling ctx stops it; builds already started keep running.
func (r *Runner) Run(ctx context.Context, m config.Macro, overrides map[string]string) error {
	st := &run{Runner: r, chained: map[string]int{}}
	for i, s := range m.Steps {
		st.step, st.total = i+1, len(m.Steps)
		st.report("▶ " + s.String())
		if err := ctx.Err(); err != nil {
			return &StepError{Step: i + 1, Desc: s.String(), Err: err}
		}
		if err := st.do(ctx, s, overrides); err != nil {
			return &StepError{Step: i + 1, Desc: s.String(), Err: err}
		}
	}
	return nil
}

type run struct {
	*Runner
	step, total int
	// chained maps a job to the build an earlier step of this run started.
	chained map[string]int
}

func (r *run) report(text string) {
	if r.Report != nil {
		r.Report(Event{Step: r.step, Total: r.total, Text: text})
	}
}

func (r *run) poll() time.Duration {
	if r.Poll > 0 {
		return r.Poll
	}
	return 5 * time.Second
}

func (r *run) sleep(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(r.poll()):
		return nil
	}
}

func (r *run) do(ctx context.Context, s config.Step, overrides map[string]string) error {
	path := jenkins.SplitFullName(s.Job)
	switch s.Kind {
	case config.StepBuild:
		return r.build(ctx, s, path, overrides)
	case config.StepAbort:
		n, err := r.target(ctx, s.Job, path)
		if err != nil {
			return err
		}
		b, err := r.Client.Build(ctx, path, n)
		if err != nil {
			return err
		}
		if !b.Building {
			return fmt.Errorf("#%d is not running", n)
		}
		if err := r.Client.Abort(ctx, path, n); err != nil {
			return err
		}
		r.report(fmt.Sprintf("✋ aborted %s #%d", s.Job, n))
		return nil
	case config.StepWait:
		return r.wait(ctx, s, path)
	case config.StepInput:
		return r.input(ctx, s, path)
	}
	return fmt.Errorf("unknown step kind %q", s.Kind)
}

func (r *run) build(ctx context.Context, s config.Step, path []string, overrides map[string]string) error {
	var params map[string]string
	if s.WithParams {
		params = maps.Clone(s.Params)
		if params == nil {
			params = map[string]string{}
		}
		for k, v := range overrides {
			if _, ok := params[k]; ok {
				params[k] = v
			}
		}
	}
	id, err := r.Client.TriggerQueued(ctx, path, params)
	if err != nil {
		return err
	}
	if id == 0 {
		r.report("🦇 queued " + s.Job + " (Jenkins gave no queue item, so later steps use its latest running build)")
		return nil
	}
	deadline := time.Now().Add(QueueTimeout)
	for {
		n, err := r.Client.QueuedBuild(ctx, id)
		if err != nil {
			return err
		}
		if n > 0 {
			r.chained[s.Job] = n
			r.report(fmt.Sprintf("🦇 started %s #%d", s.Job, n))
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("still queued after %s", QueueTimeout)
		}
		if err := r.sleep(ctx); err != nil {
			return err
		}
	}
}

var errNoRunning = errors.New("has no running build")

// target is the build started earlier in this run, else the job's newest
// running build.
func (r *run) target(ctx context.Context, job string, path []string) (int, error) {
	if n, ok := r.chained[job]; ok {
		return n, nil
	}
	d, err := r.Client.JobDetail(ctx, path)
	if err != nil {
		return 0, err
	}
	for _, b := range d.Builds {
		if b.Building {
			return b.Number, nil
		}
	}
	return 0, fmt.Errorf("%s %w", job, errNoRunning)
}

func (r *run) wait(ctx context.Context, s config.Step, path []string) error {
	n, err := r.target(ctx, s.Job, path)
	if err != nil {
		return err
	}
	deadline := time.Now().Add(time.Duration(s.Timeout()) * time.Minute)
	for {
		b, err := r.Client.Build(ctx, path, n)
		if err != nil {
			return err
		}
		if !b.Building {
			if b.Status() != jenkins.StatusSuccess {
				return fmt.Errorf("#%d finished: %s", n, b.Result)
			}
			r.report(fmt.Sprintf("✔ %s #%d succeeded", s.Job, n))
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("#%d still running after %dm (left running)", n, s.Timeout())
		}
		if err := r.sleep(ctx); err != nil {
			return err
		}
	}
}

// errNoInput skips an input step that has nothing to answer.
var errNoInput = errors.New("no pending input")

// input answers the first pending input step with its default values. On a
// build this macro started, it waits for the pipeline to get there. Unchained,
// it looks once: nothing running or nothing pending skips the step.
func (r *run) input(ctx context.Context, s config.Step, path []string) error {
	_, chained := r.chained[s.Job]
	n, err := r.target(ctx, s.Job, path)
	if err != nil {
		if !chained && errors.Is(err, errNoRunning) {
			r.report("⏭ " + s.Job + " has no running build — skipped")
			return nil
		}
		return err
	}
	if !chained {
		err := r.answer(ctx, s, path, n)
		if errors.Is(err, errNoInput) {
			r.report(fmt.Sprintf("⏭ %s #%d is not waiting for input — skipped", s.Job, n))
			return nil
		}
		return err
	}
	deadline := time.Now().Add(time.Duration(s.Timeout()) * time.Minute)
	for {
		ins, err := r.Client.PendingInputs(ctx, path, n)
		if errors.Is(err, jenkins.ErrNotFound) {
			return errors.New("can't list input steps: Jenkins lacks the Pipeline Stage View plugin")
		}
		if err != nil {
			return err
		}
		if len(ins) > 0 {
			return r.respond(ctx, s, path, n, ins[0])
		}
		b, err := r.Client.Build(ctx, path, n)
		if err != nil {
			return err
		}
		if !b.Building {
			return fmt.Errorf("#%d finished (%s) without asking for input", n, b.Result)
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("#%d asked for no input within %dm", n, s.Timeout())
		}
		if err := r.sleep(ctx); err != nil {
			return err
		}
	}
}

// answer responds to the pending input of build n, or returns errNoInput.
func (r *run) answer(ctx context.Context, s config.Step, path []string, n int) error {
	ins, err := r.Client.PendingInputs(ctx, path, n)
	if errors.Is(err, jenkins.ErrNotFound) {
		return errors.New("can't list input steps: Jenkins lacks the Pipeline Stage View plugin")
	}
	if err != nil {
		return err
	}
	if len(ins) == 0 {
		return errNoInput
	}
	return r.respond(ctx, s, path, n, ins[0])
}

func (r *run) respond(ctx context.Context, s config.Step, path []string, n int, in jenkins.InputRequest) error {
	if s.Abort {
		if err := r.Client.AbortInput(ctx, path, n, in.ID); err != nil {
			return err
		}
		r.report(fmt.Sprintf("✋ %s #%d aborted at %q", s.Job, n, in.Message))
		return nil
	}
	if err := r.Client.ProceedInput(ctx, path, n, in, defaults(in.Params)); err != nil {
		return err
	}
	r.report(fmt.Sprintf("▶ %s #%d: %s at %q", s.Job, n, in.ProceedText, in.Message))
	return nil
}

func defaults(params []jenkins.Param) map[string]string {
	out := map[string]string{}
	for _, p := range params {
		v := p.Default
		if v == "" && p.Type == jenkins.ParamChoice && len(p.Choices) > 0 {
			v = p.Choices[0]
		}
		if v == "" && p.Type == jenkins.ParamBool {
			v = "false"
		}
		out[p.Name] = v
	}
	return out
}
