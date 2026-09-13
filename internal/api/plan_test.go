package api

import (
	"testing"

	"github.com/luca-filipponi/guitar-coach/internal/model"
)

func TestBuildPlanAvoidsSameTopicAdjacency(t *testing.T) {
	pool := []model.Exercise{
		{ID: "1", Name: "a1", Topic: "A"},
		{ID: "2", Name: "a2", Topic: "A"},
		{ID: "3", Name: "a3", Topic: "A"},
		{ID: "4", Name: "b1", Topic: "B"},
		{ID: "5", Name: "b2", Topic: "B"},
		{ID: "6", Name: "n1"},
	}
	plan := BuildPlan(pool, 6)
	if len(plan) != 6 {
		t.Fatalf("expected all 6 exercises, got %d: %+v", len(plan), plan)
	}
	for i := 1; i < len(plan); i++ {
		if plan[i-1].Topic != "" && plan[i-1].Topic == plan[i].Topic {
			t.Errorf("adjacent same topic %q: %s followed by %s", plan[i-1].Topic, plan[i-1].Name, plan[i].Name)
		}
	}
}

func TestBuildPlanSelectionSpansTopics(t *testing.T) {
	pool := []model.Exercise{
		{ID: "1", Name: "a1", Topic: "A"},
		{ID: "2", Name: "a2", Topic: "A"},
		{ID: "3", Name: "a3", Topic: "A"},
		{ID: "4", Name: "b1", Topic: "B"},
		{ID: "5", Name: "b2", Topic: "B"},
		{ID: "6", Name: "c1", Topic: "C"},
		{ID: "7", Name: "c2", Topic: "C"},
	}
	plan := BuildPlan(pool, 5)
	if len(plan) != 5 {
		t.Fatalf("expected 5 exercises, got %d: %+v", len(plan), plan)
	}
	topics := map[string]bool{}
	for _, ex := range plan {
		topics[ex.Topic] = true
	}
	if !topics["B"] || !topics["C"] {
		t.Errorf("expected selection to span topics B and C, got topics %v", topics)
	}
}

func TestBuildPlanUsesAllWhenPoolSmall(t *testing.T) {
	pool := []model.Exercise{
		{ID: "1", Name: "a", Topic: "A"},
		{ID: "2", Name: "b", Topic: "A"},
		{ID: "3", Name: "c", Topic: "B"},
	}
	plan := BuildPlan(pool, 5)
	if len(plan) != 3 {
		t.Fatalf("expected all 3 exercises, got %d", len(plan))
	}
}

func TestBuildPlanDegradesGracefully(t *testing.T) {
	pool := []model.Exercise{
		{ID: "1", Name: "a1", Topic: "A"},
		{ID: "2", Name: "a2", Topic: "A"},
		{ID: "3", Name: "a3", Topic: "A"},
		{ID: "4", Name: "a4", Topic: "A"},
		{ID: "5", Name: "a5", Topic: "A"},
		{ID: "6", Name: "b1", Topic: "B"},
	}
	plan := BuildPlan(pool, 6)
	if len(plan) != 6 {
		t.Fatalf("expected all exercises, got %d", len(plan))
	}
	adjacent := 0
	for i := 1; i < len(plan); i++ {
		if plan[i-1].Topic != "" && plan[i-1].Topic == plan[i].Topic {
			adjacent++
		}
	}
	if adjacent > 4 {
		t.Errorf("expected minimal adjacency for unbalanced topics, got %d adjacent pairs", adjacent)
	}
}

func TestBuildPlanUntitledAreUnconstrained(t *testing.T) {
	pool := []model.Exercise{
		{ID: "1", Name: "a1", Topic: "A"},
		{ID: "2", Name: "a2", Topic: "A"},
		{ID: "3", Name: "n1"},
		{ID: "4", Name: "n2"},
	}
	for run := 0; run < 20; run++ {
		plan := BuildPlan(pool, 4)
		if len(plan) != 4 {
			t.Fatalf("expected all 4 exercises, got %d", len(plan))
		}
		// the only forbidden adjacency is between two titled exercises of the
		// same topic; untitled exercises may sit anywhere, even next to each other
		for i := 1; i < len(plan); i++ {
			if plan[i-1].Topic != "" && plan[i-1].Topic == plan[i].Topic {
				t.Errorf("adjacent same topic %q: %s followed by %s", plan[i-1].Topic, plan[i-1].Name, plan[i].Name)
			}
		}
	}
}
