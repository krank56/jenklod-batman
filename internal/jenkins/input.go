package jenkins

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strconv"
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
