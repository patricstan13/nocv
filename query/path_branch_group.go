package query

import (
	"fmt"
	"sort"

	"nocv/graph"
)

// PathDirection selects which end of a semantic path is grouped first.
type PathDirection uint8

const (
	// PathFromStart groups steps from the beginning of each path.
	PathFromStart PathDirection = iota
	// PathFromEnd groups steps from the end without reversing their semantics.
	PathFromEnd
)

// PathBranchLevel is one progressively requested level of exact path groups.
// Indexes always refer to the caller's original paths slice.
type PathBranchLevel struct {
	PathCount           int
	UncertainPathCount  int
	TerminalPathIndexes []int
	Branches            []PathBranch
}

// PathBranch groups selected exact paths that have the same semantic step at
// the requested depth and direction.
type PathBranch struct {
	Step               SemanticStep
	PathCount          int
	UncertainPathCount int
	PathIndexes        []int
}

// GroupPathBranches partitions selected exact paths by one semantic step.
// A nil pathIndexes slice selects every path. Invalid programmer arguments
// panic; returned indexes are ephemeral selectors tied to the paths slice and its order.
func GroupPathBranches(
	paths []SemanticPath,
	pathIndexes []int,
	depth int,
	direction PathDirection,
) PathBranchLevel {
	if depth < 0 {
		panic("query: path branch depth must be non-negative")
	}
	if direction != PathFromStart && direction != PathFromEnd {
		panic("query: invalid path branch direction")
	}

	selected := selectedPathIndexes(len(paths), pathIndexes)
	if len(selected) == 0 {
		return PathBranchLevel{}
	}

	level := PathBranchLevel{PathCount: len(selected)}
	branchesByStep := make(map[SemanticStep]*PathBranch)
	for _, pathIndex := range selected {
		path := paths[pathIndex]
		uncertain := semanticPathIsUncertain(path)
		if uncertain {
			level.UncertainPathCount++
		}

		stepIndex := depth
		if direction == PathFromEnd {
			stepIndex = len(path.Steps) - 1 - depth
		}
		if stepIndex < 0 || stepIndex >= len(path.Steps) {
			level.TerminalPathIndexes = append(level.TerminalPathIndexes, pathIndex)
			continue
		}

		step := path.Steps[stepIndex]
		branch := branchesByStep[step]
		if branch == nil {
			branch = &PathBranch{Step: step}
			branchesByStep[step] = branch
		}
		branch.PathCount++
		if uncertain {
			branch.UncertainPathCount++
		}
		branch.PathIndexes = append(branch.PathIndexes, pathIndex)
	}

	level.Branches = make([]PathBranch, 0, len(branchesByStep))
	for _, branch := range branchesByStep {
		level.Branches = append(level.Branches, *branch)
	}
	sort.Slice(level.Branches, func(i, j int) bool {
		return pathBranchStepLess(level.Branches[i].Step, level.Branches[j].Step)
	})
	return level
}

func selectedPathIndexes(pathCount int, indexes []int) []int {
	if indexes == nil {
		selected := make([]int, pathCount)
		for index := range selected {
			selected[index] = index
		}
		return selected
	}

	selected := append([]int(nil), indexes...)
	seen := make(map[int]bool, len(selected))
	for _, index := range selected {
		if index < 0 || index >= pathCount {
			panic(fmt.Sprintf("query: path index %d out of range", index))
		}
		if seen[index] {
			panic(fmt.Sprintf("query: duplicate path index %d", index))
		}
		seen[index] = true
	}
	sort.Ints(selected)
	return selected
}

func semanticPathIsUncertain(path SemanticPath) bool {
	uncertain := false
	for _, step := range path.Steps {
		switch step.Certainty {
		case graph.RelationshipConfirmed:
		case graph.RelationshipUncertain:
			uncertain = true
		default:
			panic("query: semantic path contains invalid relationship certainty")
		}
	}
	return uncertain
}

func pathBranchStepLess(left, right SemanticStep) bool {
	if left.From != right.From {
		return left.From < right.From
	}
	if left.Kind != right.Kind {
		return left.Kind < right.Kind
	}
	if left.To != right.To {
		return left.To < right.To
	}
	return relationshipCertaintyRank(left.Certainty) < relationshipCertaintyRank(right.Certainty)
}
