package engine

import (
	"job-o-matic/model"
	"testing"
)

func TestBuildLevels(t *testing.T) {
	steps := []*model.Step{
		&model.Step{Id: "a"},
		&model.Step{Id: "b", DependsOn: []string{"a"}},
		&model.Step{Id: "c", DependsOn: []string{"a"}},
		&model.Step{Id: "d", DependsOn: []string{"c"}},
	}

	levels, err := BuildLevels(steps)

	if err != nil {
		t.Fatalf("Expected nil err, got: %v", err)
	}

	if len(levels) != 3 {
		t.Fatalf("expected 3 levels got %d", len(levels))
	}

	if len(levels[0]) != 1 {
		t.Fatalf("expected level 1 to have 1 step got %d", len(levels))
	}

	if len(levels[1]) != 2 {
		t.Fatalf("expected level 2 to have 2 steps got %d", len(levels))
	}

	if len(levels[2]) != 1 {
		t.Fatalf("expected level 3 to have 1 step got %d", len(levels))
	}
}

func TestBuildLevelsCircularDependency(t *testing.T) {
	steps := []*model.Step{
		&model.Step{Id: "a", DependsOn: []string{"b"}},
		&model.Step{Id: "b", DependsOn: []string{"a"}},
	}

	_, err := BuildLevels(steps)

	if err == nil {
		t.Fatalf("Expected err, got: %v", err)
	}
}
