package config

import (
	"fmt"
	"maps"
	"slices"
	"strings"
)

// Step kinds.
const (
	StepBuild = "build" // trigger a job
	StepAbort = "abort" // abort its running build
	StepWait  = "wait"  // wait for a build to finish successfully
	StepInput = "input" // answer a pending input step
)

// DefaultTimeoutMinutes bounds wait and input steps.
const DefaultTimeoutMinutes = 30

// Macro is a named sequence of steps, run with `m` or `run <name>`.
type Macro struct {
	Name string `toml:"name"`
	// Arg names the build parameter that a value typed after the macro's
	// name fills: with Arg = "BRANCH", `jenklod-batman ship main` runs it
	// with BRANCH=main.
	Arg   string `toml:"arg,omitempty"`
	Steps []Step `toml:"steps"`
}

// BuiltJobs lists the jobs the macro's build steps start, once each.
func (m Macro) BuiltJobs() []string {
	var out []string
	for _, s := range m.Steps {
		if s.Kind == StepBuild && !slices.Contains(out, s.Job) {
			out = append(out, s.Job)
		}
	}
	return out
}

// HasParam reports whether a build step of the macro passes parameter k.
func (m Macro) HasParam(k string) bool {
	for _, s := range m.Steps {
		if _, ok := s.Params[k]; ok && s.Kind == StepBuild && s.WithParams {
			return true
		}
	}
	return false
}

// Step is one action of a macro.
type Step struct {
	Kind string `toml:"kind"`
	Job  string `toml:"job"` // full name
	// WithParams triggers buildWithParameters with Params; otherwise the
	// job is started plainly. Password parameters are never saved.
	WithParams bool              `toml:"with_params,omitempty"`
	Params     map[string]string `toml:"params,omitempty"`
	// TimeoutMinutes bounds wait and input steps (0 means the default).
	TimeoutMinutes int `toml:"timeout_minutes,omitempty"`
	// Abort answers an input step with "Abort" instead of proceeding.
	Abort bool `toml:"abort,omitempty"`
}

// Timeout is the effective timeout in minutes.
func (s Step) Timeout() int {
	if s.TimeoutMinutes > 0 {
		return s.TimeoutMinutes
	}
	return DefaultTimeoutMinutes
}

// String describes the step for lists and progress output.
func (s Step) String() string {
	switch s.Kind {
	case StepBuild:
		if !s.WithParams || len(s.Params) == 0 {
			return "build " + s.Job
		}
		var kv []string
		for _, k := range slices.Sorted(maps.Keys(s.Params)) {
			kv = append(kv, k+"="+s.Params[k])
		}
		return fmt.Sprintf("build %s (%s)", s.Job, strings.Join(kv, ", "))
	case StepAbort:
		return "abort running build of " + s.Job
	case StepWait:
		return fmt.Sprintf("wait for %s to succeed (%dm)", s.Job, s.Timeout())
	case StepInput:
		answer := "proceed"
		if s.Abort {
			answer = "abort"
		}
		return fmt.Sprintf("%s input on %s (%dm)", answer, s.Job, s.Timeout())
	}
	return s.Kind + " " + s.Job
}
