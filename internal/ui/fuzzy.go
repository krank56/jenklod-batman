package ui

import (
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/krank56/jenklod-batman/internal/jenkins"
)

// fuzzyScore matches pat as a subsequence of s, ignoring case, fzf style.
// Consecutive runs and matches at word starts ("/", "-", "_", ".", " ")
// score higher, as does matching within the last path segment.
func fuzzyScore(pat, s string) (int, bool) {
	p := []rune(strings.ToLower(pat))
	r := []rune(strings.ToLower(s))
	if len(p) == 0 {
		return 0, true
	}
	lastSeg := strings.LastIndexByte(s, '/') + 1
	lastSeg = utf8.RuneCountInString(s[:lastSeg])
	best, found := 0, false
	// Try each place the first rune occurs; greedy from there.
	for start := range r {
		if r[start] != p[0] {
			continue
		}
		score, pi, prev := 0, 0, -2
		for i := start; i < len(r) && pi < len(p); i++ {
			if r[i] != p[pi] {
				continue
			}
			score++
			if i == prev+1 {
				score += 5
			}
			if i == 0 || strings.ContainsRune("/-_. ", r[i-1]) {
				score += 8
			}
			if i >= lastSeg {
				score += 2
			}
			if prev >= 0 {
				score -= min(i-prev-1, 3)
			}
			prev = i
			pi++
		}
		if pi == len(p) && (!found || score > best) {
			best, found = score, true
		}
	}
	return best, found
}

// rankJobs keeps the jobs whose key matches pat, best first.
func rankJobs(pat string, jobs []jenkins.Job, key func(jenkins.Job) string) []jenkins.Job {
	type hit struct {
		job   jenkins.Job
		score int
	}
	var hits []hit
	for _, j := range jobs {
		if s, ok := fuzzyScore(pat, key(j)); ok {
			hits = append(hits, hit{j, s})
		}
	}
	sort.SliceStable(hits, func(a, b int) bool {
		if hits[a].score != hits[b].score {
			return hits[a].score > hits[b].score
		}
		return len(key(hits[a].job)) < len(key(hits[b].job))
	})
	out := make([]jenkins.Job, len(hits))
	for i, h := range hits {
		out[i] = h.job
	}
	return out
}
