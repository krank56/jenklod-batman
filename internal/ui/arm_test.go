package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/krank56/jenklod-batman/internal/jenkins"
)

func TestPreAnswerFlow(t *testing.T) {
	cfg := newCfg(t)
	m, fj := start(t, cfg)
	m = drive(t, m, key("enter")) // gotham/
	m = drive(t, m, key("enter")) // deploy: #42 runs, not waiting
	m = drive(t, m, key("i"))
	if m.arm == nil {
		t.Fatal("i on a running build should open the pre-answer dialog")
	}
	v := snapshot(t, "30-arm", m)
	for _, want := range []string{"Pre-answer gotham/deploy #42", "Last time (#41)", "Deploy to prod?", "Bruce Wayne", "‹ Deploy ›"} {
		if !strings.Contains(v, want) {
			t.Errorf("dialog lacks %q:\n%s", want, v)
		}
	}
	m = drive(t, m, key("enter"))
	a := m.armed["gotham/deploy#42"]
	if a == nil || !a.proceed || a.expect != "Deploy to prod?" {
		t.Fatalf("armed = %+v", a)
	}
	if v := snapshot(t, "31-armed", m); !strings.Contains(v, `⏵ armed: Deploy if asked "Deploy to prod?"`) {
		t.Errorf("the build row lacks the armed mark:\n%s", v)
	}

	// It pauses: the next poll answers with the step's default.
	fj.mu.Lock()
	fj.withInput = true
	fj.mu.Unlock()
	m = drive(t, m, cmdMsg{m.pollArmed()})
	if len(m.armed) != 0 {
		t.Error("an answer fired must disarm")
	}
	if !strings.Contains(m.status, "Auto-approved gotham/deploy #42") {
		t.Errorf("status = %q", m.status)
	}
	fj.mu.Lock()
	defer fj.mu.Unlock()
	want := `/job/gotham/job/deploy/42/input/Prod/submit {"parameter":[{"name":"REGION","value":"eu"}]}`
	if strings.Join(fj.answered, "\n") != want {
		t.Errorf("answered:\n%s\nwant:\n%s", strings.Join(fj.answered, "\n"), want)
	}
}

func TestPreAnswerDisarmAndQuit(t *testing.T) {
	cfg := newCfg(t)
	m, _ := start(t, cfg)
	m = drive(t, m, key("enter"))
	m = drive(t, m, key("enter"))
	m = drive(t, m, key("i"))
	m = drive(t, m, key("right")) // Abort
	m = drive(t, m, key("enter"))
	if a := m.armed["gotham/deploy#42"]; a == nil || a.proceed {
		t.Fatalf("armed = %+v", a)
	}
	m = drive(t, m, key("h"))
	m = drive(t, m, key("h"))
	next, cmd := m.Update(key("q"))
	m = next.(Model)
	if cmd != nil || m.confirm == nil || !strings.Contains(m.confirm.prompt, "lost on quit") {
		t.Fatal("q must ask first while an answer is armed")
	}
	m = drive(t, m, key("n"))
	m = drive(t, m, key("enter"))
	m = drive(t, m, key("enter"))
	m = drive(t, m, key("i"))
	if m.arm == nil || !m.arm.existing {
		t.Fatal("i on an armed build shows it")
	}
	m = drive(t, m, key("d"))
	if len(m.armed) != 0 {
		t.Error("d must disarm")
	}
}

func TestPreAnswerRefusesAnotherQuestion(t *testing.T) {
	m := New(Options{Config: newCfg(t), NoAnim: true})
	job := jenkins.Job{Name: "deploy", Path: []string{"gotham", "deploy"}}
	m.armed["gotham/deploy#9"] = &armed{job: job, number: 9, proceed: true, expect: "Deploy to staging?"}
	b := &jenkins.Build{Number: 9, Building: true, Inputs: []jenkins.InputRequest{{ID: "Prod", Message: "Deploy to prod?"}}}
	if m.armedSaw("gotham/deploy", b) == nil {
		t.Error("a different question must notify")
	}
	if len(m.armed) != 0 || !strings.Contains(m.status, `asks "Deploy to prod?"`) {
		t.Errorf("armed=%d status=%q", len(m.armed), m.status)
	}
	// The watch poll does not notify it a second time.
	m.cfg.TogglePin("gotham/deploy")
	if m.handleWatch(watchResultMsg{name: "gotham/deploy", build: b}) != nil {
		t.Error("the input was already said")
	}
}

func TestPreAnswerFinishedWithoutAsking(t *testing.T) {
	m := New(Options{Config: newCfg(t), NoAnim: true})
	m.armed["lint#3"] = &armed{job: jenkins.Job{Name: "lint", Path: []string{"lint"}}, number: 3, proceed: true}
	if m.armedSaw("lint", &jenkins.Build{Number: 3, Building: true}) != nil || len(m.armed) != 1 {
		t.Error("running, not paused: nothing to do")
	}
	if m.armedSaw("lint", &jenkins.Build{Number: 3, Result: "SUCCESS"}) == nil || len(m.armed) != 0 {
		t.Error("finished without asking must notify and disarm")
	}
}

func TestInputDefaults(t *testing.T) {
	vals, missing := inputDefaults([]jenkins.Param{
		{Name: "REGION", Type: jenkins.ParamChoice, Choices: []string{"eu", "us"}},
		{Name: "FORCE", Type: jenkins.ParamBool},
		{Name: "TAG", Default: "v1"},
	})
	if missing != "" || vals["REGION"] != "eu" || vals["FORCE"] != "false" || vals["TAG"] != "v1" {
		t.Errorf("vals=%v missing=%q", vals, missing)
	}
	if _, missing := inputDefaults([]jenkins.Param{{Name: "WHY"}}); missing != "WHY" {
		t.Errorf("a text field without default: missing=%q", missing)
	}
	if _, missing := inputDefaults([]jenkins.Param{{Name: "PW", Type: jenkins.ParamPassword, Default: "x"}}); missing != "PW" {
		t.Errorf("a password: missing=%q", missing)
	}
}

var _ tea.Model = Model{}
