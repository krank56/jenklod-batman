package jenkins

import "testing"

func TestParsePastInput(t *testing.T) {
	for _, tc := range []struct {
		name, log string
		want      PastInput
		ok        bool
	}{
		{"approved", "[Pipeline] stage\n[Pipeline] input\nDeploy to prod?\nDeploy or Abort\nApproved by Bruce Wayne\n[Pipeline] }\n",
			PastInput{Message: "Deploy to prod?", ProceedText: "Deploy", Approved: true, By: "Bruce Wayne"}, true},
		{"aborted", "[Pipeline] input\nShip it?\nProceed or Abort\r\nAborted by Joker\n",
			PastInput{Message: "Ship it?", ProceedText: "Proceed", By: "Joker"}, true},
		{"multi-line message", "[Pipeline] input\nRelease 1.2\nto prod?\nGo or Abort\nApproved by alfred\n",
			PastInput{Message: "Release 1.2\nto prod?", ProceedText: "Go", Approved: true, By: "alfred"}, true},
		{"first of two", "[Pipeline] input\nStaging?\nProceed or Abort\nApproved by a\n[Pipeline] input\nProd?\nProceed or Abort\nAborted by b\n",
			PastInput{Message: "Staging?", ProceedText: "Proceed", Approved: true, By: "a"}, true},
		{"never answered", "[Pipeline] input\nProd?\nProceed or Abort\nTimeout has been exceeded\n", PastInput{}, false},
		{"no input", "Started\nFinished: SUCCESS\n", PastInput{}, false},
	} {
		got, ok := ParsePastInput(tc.log)
		if ok != tc.ok || got != tc.want {
			t.Errorf("%s: got %+v %v, want %+v %v", tc.name, got, ok, tc.want, tc.ok)
		}
	}
}
