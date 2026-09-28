package demo

import (
	"context"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/krank56/jenklod-batman/internal/config"
	"github.com/krank56/jenklod-batman/internal/jenkins"
	"github.com/krank56/jenklod-batman/internal/macro"
)

// clock is a fake time the test moves by hand.
type clock struct {
	mu sync.Mutex
	t  time.Time
}

func (c *clock) now() time.Time { c.mu.Lock(); defer c.mu.Unlock(); return c.t }
func (c *clock) add(d time.Duration) {
	c.mu.Lock()
	c.t = c.t.Add(d)
	c.mu.Unlock()
}

func setup(t *testing.T) (*Server, *jenkins.Client, *clock) {
	t.Helper()
	s := New()
	clk := &clock{t: time.Now()}
	s.mu.Lock()
	s.now = clk.now
	s.mu.Unlock()
	srv := httptest.NewServer(s)
	t.Cleanup(srv.Close)
	c, err := jenkins.New(srv.URL, "bruce", "demo")
	if err != nil {
		t.Fatal(err)
	}
	return s, c, clk
}

func TestDemoTreeAndDetail(t *testing.T) {
	_, c, _ := setup(t)
	ctx := context.Background()
	if who, err := c.Ping(ctx); err != nil || who != "Bruce Wayne" {
		t.Fatalf("ping = %q, %v", who, err)
	}
	all, err := c.AllJobs(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, j := range all {
		names = append(names, j.FullName())
	}
	joined := strings.Join(names, ",")
	for _, want := range []string{"gotham/deploy", "wayne-enterprises/applied-sciences/grappling-hook-ci", "batmobile-lint"} {
		if !strings.Contains(joined, want) {
			t.Errorf("tree lacks %s: %s", want, joined)
		}
	}
	d, err := c.JobDetail(ctx, []string{"gotham", "deploy"})
	if err != nil {
		t.Fatal(err)
	}
	if len(d.Params) != 3 || !d.Builds[0].Building || d.Builds[1].Result != "SUCCESS" {
		t.Errorf("detail = %+v", d)
	}
}

func TestDemoBuildLifecycle(t *testing.T) {
	_, c, clk := setup(t)
	ctx := context.Background()
	path := []string{"gotham", "deploy"}

	id, err := c.TriggerQueued(ctx, path, map[string]string{"ENV": "prod", "DRY_RUN": "false", "VERSION": "v3"})
	if err != nil || id == 0 {
		t.Fatalf("trigger: id=%d err=%v", id, err)
	}
	if n, _ := c.QueuedBuild(ctx, id); n != 0 {
		t.Fatalf("should still be queued, got #%d", n)
	}
	clk.add(2 * time.Second)
	n, err := c.QueuedBuild(ctx, id)
	if err != nil || n == 0 {
		t.Fatalf("queued build: n=%d err=%v", n, err)
	}

	// The log only grows, and pauses at the input.
	var log strings.Builder
	var offset int64
	read := func() {
		t.Helper()
		ch, err := c.Log(ctx, path, n, offset)
		if err != nil {
			t.Fatal(err)
		}
		if ch.Next < offset {
			t.Fatalf("log shrank: %d < %d", ch.Next, offset)
		}
		log.WriteString(ch.Text)
		offset = ch.Next
	}
	read()
	clk.add(30 * time.Second)
	read()
	ins, err := c.PendingInputs(ctx, path, n)
	if err != nil || len(ins) != 1 || ins[0].Message != "Deploy to prod?" {
		t.Fatalf("inputs = %+v, %v", ins, err)
	}
	if b, _ := c.Build(ctx, path, n); !b.Building {
		t.Fatal("a paused build is still building")
	}
	if err := c.ProceedInput(ctx, path, n, ins[0], nil); err != nil {
		t.Fatal(err)
	}
	clk.add(10 * time.Second)
	read()
	b, _ := c.Build(ctx, path, n)
	if b.Building || b.Result != "SUCCESS" {
		t.Fatalf("build = %+v", b)
	}
	text := log.String()
	for _, want := range []string{"ENV = prod", "Deploy to prod?", "Approved by Bruce Wayne", "12/12 pods ready", "Finished: SUCCESS"} {
		if !strings.Contains(text, want) {
			t.Errorf("log lacks %q:\n%s", want, text)
		}
	}
}

func TestDemoRunsAMacro(t *testing.T) {
	_, c, clk := setup(t)
	// Time flies whenever the runner waits.
	r := &macro.Runner{Client: c, Poll: time.Millisecond, Report: func(macro.Event) { clk.add(3 * time.Second) }}
	m := config.Macro{Name: "ship", Steps: []config.Step{
		{Kind: config.StepAbort, Job: "gotham/deploy"}, // the deploy already running
		{Kind: config.StepBuild, Job: "gotham/deploy", WithParams: true, Params: map[string]string{"ENV": "prod", "DRY_RUN": "false", "VERSION": "v3"}},
		{Kind: config.StepInput, Job: "gotham/deploy"},
		{Kind: config.StepWait, Job: "gotham/deploy"},
	}}
	done := make(chan error, 1)
	go func() { done <- r.Run(context.Background(), m, nil) }()
	for {
		select {
		case err := <-done:
			if err != nil {
				t.Fatal(err)
			}
			return
		case <-time.After(time.Millisecond):
			clk.add(time.Second)
		}
	}
}
