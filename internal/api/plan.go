package api

import (
	"math/rand"
	"sort"

	"github.com/luca-filipponi/guitar-coach/internal/model"
)

// BuildPlan picks n exercises from pool and orders them so that exercises of
// the same topic are kept apart. Topics are balanced across the selection via
// round-robin, then the result is arranged to avoid adjacent same-topic
// exercises (degrading gracefully when a topic dominates the pool).
func BuildPlan(pool []model.Exercise, n int) []model.Exercise {
	if n < 1 {
		n = 5
	}
	if len(pool) <= n {
		return arrange(pool)
	}
	return arrange(selectByTopic(pool, n))
}

// ScrambleRotation reorders a round's exercises for the next round. The set is
// unchanged, adjacent exercises of the same topic are kept apart, and the
// result is neither the previous order nor its exact reverse, so a rotation
// never simply mirrors itself.
func ScrambleRotation(in []model.Exercise) []model.Exercise {
	out := append([]model.Exercise(nil), in...)
	if len(in) < 2 {
		return out
	}
	rev := make([]model.Exercise, len(in))
	for i := range in {
		rev[len(in)-1-i] = in[i]
	}
	for attempt := 0; attempt < 12; attempt++ {
		tmp := append([]model.Exercise(nil), in...)
		rand.Shuffle(len(tmp), func(i, j int) {
			tmp[i], tmp[j] = tmp[j], tmp[i]
		})
		candidate := arrange(tmp)
		if !sameOrder(candidate, in) && !sameOrder(candidate, rev) {
			return candidate
		}
	}
	candidate := arrange(append([]model.Exercise(nil), in...))
	for shift := 1; shift < len(in); shift++ {
		rot := append(candidate[shift:], candidate[:shift]...)
		if !sameOrder(rot, in) && !sameOrder(rot, rev) {
			return rot
		}
	}
	return rev
}

func sameOrder(a, b []model.Exercise) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].ID != b[i].ID {
			return false
		}
	}
	return true
}

// selectByTopic picks n exercises from pool, taking at most one exercise per
// topic in each pass so no two adjacent picks share a topic.
func selectByTopic(pool []model.Exercise, n int) []model.Exercise {
	groups := groupByTopic(pool)

	keys := make([]string, 0, len(groups))
	for k := range groups {
		keys = append(keys, k)
	}
	rand.Shuffle(len(keys), func(i, j int) {
		keys[i], keys[j] = keys[j], keys[i]
	})
	for _, k := range keys {
		rand.Shuffle(len(groups[k]), func(i, j int) {
			groups[k][i], groups[k][j] = groups[k][j], groups[k][i]
		})
	}

	var selected []model.Exercise
	for len(selected) < n && len(keys) > 0 {
		var next []string
		for _, k := range keys {
			g := groups[k]
			if len(g) == 0 {
				continue
			}
			selected = append(selected, g[0])
			groups[k] = g[1:]
			if len(selected) == n {
				break
			}
			next = append(next, k)
		}
		keys = next
	}
	return selected
}

// arrange reorders exercises so adjacent entries never share a topic, taking
// from the largest remaining topic first (falling back when a topic dominates).
func arrange(in []model.Exercise) []model.Exercise {
	if len(in) < 2 {
		return in
	}
	groups := groupByTopic(in)
	keys := make([]string, 0, len(groups))
	for k := range groups {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		return len(groups[keys[i]]) > len(groups[keys[j]])
	})

	idx := make(map[string]int, len(keys))
	out := make([]model.Exercise, 0, len(in))
	last := ""
	for len(out) < len(in) {
		best := ""
		for _, k := range keys {
			if k != last && idx[k] < len(groups[k]) {
				best = k
				break
			}
		}
		if best == "" {
			for _, k := range keys {
				if idx[k] < len(groups[k]) {
					best = k
					break
				}
			}
		}
		out = append(out, groups[best][idx[best]])
		idx[best]++
		last = best
	}
	return out
}

// groupByTopic buckets exercises by topic. Untitled exercises have no
// constraint, so each gets its own group: the planner can place them anywhere
// (even adjacent) without ever wasting a slot to keep them apart.
func groupByTopic(exercises []model.Exercise) map[string][]model.Exercise {
	groups := make(map[string][]model.Exercise)
	for _, ex := range exercises {
		key := ex.Topic
		if key == "" {
			key = "\x00" + ex.ID
		}
		groups[key] = append(groups[key], ex)
	}
	return groups
}
