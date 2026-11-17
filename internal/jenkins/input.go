package jenkins

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"
)

// InputRequest is a pipeline `input` step waiting for someone to proceed
// or abort, typically "Deploy to production?".
type InputRequest struct {
	ID          string
	Message     string
	ProceedText string
	Params      []Param
}

// PendingInputs lists the input steps a running pipeline build is paused on.
// It relies on the Pipeline: Stage View plugin (wfapi); without it the error
// wraps ErrNotFound.
func (c *Client) PendingInputs(ctx context.Context, path []string, number int) ([]InputRequest, error) {
	var resp []struct {
		ID          string `json:"id"`
		Message     string `json:"message"`
		ProceedText string `json:"proceedText"`
		Inputs      []struct {
			Type        string `json:"type"`
			Name        string `json:"name"`
			Description string `json:"description"`
			Definition  struct {
				DefaultVal any      `json:"defaultVal"`
				Choices    []string `json:"choices"`
			} `json:"definition"`
		} `json:"inputs"`
	}
	p := jobPath(path) + "/" + strconv.Itoa(number) + "/wfapi/pendingInputActions"
	if err := c.getRaw(ctx, p, nil, &resp); err != nil {
		return nil, err
	}
	out := make([]InputRequest, 0, len(resp))
	for _, r := range resp {
		in := InputRequest{ID: r.ID, Message: r.Message, ProceedText: r.ProceedText}
		if in.ProceedText == "" {
			in.ProceedText = "Proceed"
		}
		for _, d := range r.Inputs {
			p := Param{Name: d.Name, Description: d.Description, Type: paramType(d.Type), Choices: d.Definition.Choices}
			if d.Definition.DefaultVal != nil && p.Type != ParamPassword {
				p.Default = fmt.Sprint(d.Definition.DefaultVal)
			}
			in.Params = append(in.Params, p)
		}
		out = append(out, in)
	}
	return out, nil
}

func inputPath(path []string, number int, id string) string {
	return jobPath(path) + "/" + strconv.Itoa(number) + "/input/" + url.PathEscape(id)
}

// ProceedInput answers an input step. values holds one string per
// parameter; booleans are "true"/"false".
func (c *Client) ProceedInput(ctx context.Context, path []string, number int, in InputRequest, values map[string]string) error {
	if len(in.Params) == 0 {
		return c.post(ctx, inputPath(path, number, in.ID)+"/proceedEmpty", nil)
	}
	type pv struct {
		Name  string `json:"name"`
		Value any    `json:"value"`
	}
	var params []pv
	for _, p := range in.Params {
		var v any = values[p.Name]
		if p.Type == ParamBool {
			v = values[p.Name] == "true"
		}
		params = append(params, pv{p.Name, v})
	}
	js, err := json.Marshal(map[string]any{"parameter": params})
	if err != nil {
		return err
	}
	form := url.Values{"json": {string(js)}, "proceed": {in.ProceedText}}
	return c.post(ctx, inputPath(path, number, in.ID)+"/submit", form)
}

// AbortInput rejects an input step, which aborts the build.
func (c *Client) AbortInput(ctx context.Context, path []string, number int, id string) error {
	return c.post(ctx, inputPath(path, number, id)+"/abort", nil)
}

// IsPipeline reports whether the job can pause on input steps. Jobs whose
// class is unknown (e.g. a pinned job never listed) are assumed to be.
func (j Job) IsPipeline() bool {
	return j.Class == "" || j.Class == "org.jenkinsci.plugins.workflow.job.WorkflowJob"
}

// AttachInputs fills Inputs on the running builds. It returns false when the
// controller lacks the Stage View API, in which case inputs cannot be seen.
func (c *Client) AttachInputs(ctx context.Context, path []string, builds []Build) (bool, error) {
	const maxRunning = 5 // a job rarely has more concurrent builds worth checking
	checked := 0
	for i := range builds {
		if !builds[i].Building || checked == maxRunning {
			continue
		}
		checked++
		ins, err := c.PendingInputs(ctx, path, builds[i].Number)
		if errors.Is(err, ErrNotFound) {
			return false, nil
		}
		if err != nil {
			return true, err
		}
		builds[i].Inputs = ins
	}
	return true, nil
}

// PastInput is what a finished build's console says about the first input
// step it paused on and how it was answered. Jenkins keeps no other record
// of it that the REST API returns once the build is over.
type PastInput struct {
	Number      int // the build it was read from
	Message     string
	ProceedText string
	Approved    bool   // false: aborted
	By          string // who answered, as the console names them
}

// inputPrompt is the line the input step prints under its message.
var inputPrompt = regexp.MustCompile(`^(.+) or Abort$`)

// ParsePastInput finds the first answered input step in a console log:
//
//	[Pipeline] input
//	Deploy to prod?
//	Deploy or Abort
//	Approved by Bruce Wayne
func ParsePastInput(log string) (PastInput, bool) {
	lines := strings.Split(strings.ReplaceAll(log, "\r", ""), "\n")
	for i, l := range lines {
		m := inputPrompt.FindStringSubmatch(strings.TrimSpace(l))
		if m == nil || i == 0 {
			continue
		}
		// The message is everything since "[Pipeline] input", or the line
		// above the prompt when that marker is missing.
		first := i - 1
		for j := i - 1; j >= 0 && i-j <= 20; j-- {
			if strings.TrimSpace(lines[j]) == "[Pipeline] input" {
				first = j + 1
				break
			}
		}
		msg := strings.TrimSpace(strings.Join(lines[first:i], "\n"))
		if msg == "" {
			continue
		}
		for _, after := range lines[i+1:] {
			after = strings.TrimSpace(after)
			if strings.HasPrefix(after, "[Pipeline] input") {
				break // the next input step: this one was never answered
			}
			for prefix, ok := range map[string]bool{"Approved by ": true, "Aborted by ": false, "Rejected by ": false} {
				if by, found := strings.CutPrefix(after, prefix); found {
					return PastInput{Message: msg, ProceedText: m[1], Approved: ok, By: by}, true
				}
			}
		}
	}
	return PastInput{}, false
}

// LastInput reads the consoles of builds, newest first, and returns the
// first answered input step found.
func (c *Client) LastInput(ctx context.Context, path []string, builds []int) (PastInput, bool, error) {
	for _, n := range builds {
		chunk, err := c.Log(ctx, path, n, 0)
		if err != nil {
			return PastInput{}, false, err
		}
		if p, ok := ParsePastInput(chunk.Text); ok {
			p.Number = n
			return p, true, nil
		}
	}
	return PastInput{}, false, nil
}
