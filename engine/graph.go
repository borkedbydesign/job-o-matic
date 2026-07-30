package engine

import (
	"errors"
	"fmt"
	"job-o-matic/model"
)

func BuildLevels(steps []*model.Step) ([][]*model.Step, error) {
	graph, err := buildGraph(steps)
	if err != nil {
		return nil, err
	}

	inDegree := findInDegree(graph)

	stepById := make(map[string]*model.Step, len(steps))
	for _, s := range steps {
		stepById[s.Id] = s
	}

	level := make(map[string]int)
	remaining := make(map[string]int)
	for id, deg := range inDegree {
		remaining[id] = deg
	}

	queue := make([]string, 0)

	for id, deg := range inDegree {
		if deg == 0 {
			level[id] = 0
			queue = append(queue, id)
		}
	}

	processed := 0
	for len(queue) > 0 {
		currentId := queue[0]
		queue = queue[1:]
		processed++

		currentLevel := level[currentId]

		for _, childId := range graph[currentId] {
			proposed := currentLevel + 1
			if existing, ok := level[childId]; !ok || proposed > existing {
				level[childId] = proposed
			}

			remaining[childId]--
			if remaining[childId] == 0 {
				queue = append(queue, childId)
			}
		}
	}

	if processed != len(steps) {
		return nil, errors.New("cycle detected or dangling dependency in workflow")
	}

	levels := make(map[int][]*model.Step)
	for id, lvl := range level {
		levels[lvl] = append(levels[lvl], stepById[id])
	}

	maxLevel := -1
	for lvl := range levels {
		if lvl > maxLevel {
			maxLevel = lvl
		}
	}
	ordered := make([][]*model.Step, maxLevel+1)
	for lvl, s := range levels {
		ordered[lvl] = s
	}
	return ordered, nil
}

func buildGraph(steps []*model.Step) (map[string][]string, error) {
	graph := make(map[string][]string, len(steps))

	for _, step := range steps {
		graph[step.Id] = []string{}
	}

	for _, step := range steps {
		for _, parentId := range step.DependsOn {
			if _, ok := graph[parentId]; !ok {
				return nil, fmt.Errorf("dependency of step '%s' does not exist", step.Id)
			}
			graph[parentId] = append(graph[parentId], step.Id)
		}
	}
	return graph, nil
}

func findInDegree(graph map[string][]string) map[string]int {
	inDegree := make(map[string]int, len(graph))
	for node := range graph {
		inDegree[node] = 0
	}
	for _, neighbours := range graph {
		for _, neighbour := range neighbours {
			inDegree[neighbour]++
		}
	}
	return inDegree
}
