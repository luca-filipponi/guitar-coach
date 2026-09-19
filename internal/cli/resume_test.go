package cli

import (
	"reflect"
	"testing"

	"github.com/luca-filipponi/guitar-coach/internal/model"
)

func TestRoundOrderFromEntries(t *testing.T) {
	sess := model.Session{
		Entries: []model.Entry{
			{Round: 1, Sequence: 3, ExerciseID: "ex_3"},
			{Round: 1, Sequence: 1, ExerciseID: "ex_1"},
			{Round: 2, Sequence: 2, ExerciseID: "ex_5"},
			{Round: 1, Sequence: 4, ExerciseID: "ex_4"},
			{Round: 2, Sequence: 1, ExerciseID: "ex_2"},
		},
	}
	order, seq := roundOrderFromEntries(sess, 1)
	if want := []string{"ex_1", "ex_3", "ex_4"}; !reflect.DeepEqual(order, want) {
		t.Fatalf("round 1 order = %v, want %v", order, want)
	}
	if seq != 3 {
		t.Fatalf("round 1 completed = %d, want 3", seq)
	}
	order2, seq2 := roundOrderFromEntries(sess, 2)
	if want := []string{"ex_2", "ex_5"}; !reflect.DeepEqual(order2, want) {
		t.Fatalf("round 2 order = %v, want %v", order2, want)
	}
	if seq2 != 2 {
		t.Fatalf("round 2 completed = %d, want 2", seq2)
	}
}
