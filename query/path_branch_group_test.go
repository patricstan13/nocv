package query_test

import (
	"fmt"
	"reflect"
	"slices"
	"sort"
	"testing"

	"nocv/graph"
	"nocv/query"
)

func TestGroupPathBranchesGroupsForwardAndExpandsSubset(t *testing.T) {
	paths := []query.SemanticPath{
		semanticPath(confirmedStep("A", "B"), confirmedStep("B", "D")),
		semanticPath(confirmedStep("A", "B"), confirmedStep("B", "E")),
		semanticPath(confirmedStep("A", "C"), confirmedStep("C", "E")),
	}

	level := query.GroupPathBranches(paths, nil, 0, query.PathFromStart)
	assertLevelAccounting(t, level, nil, len(paths))
	assertBranch(t, level.Branches[0], confirmedStep("A", "B"), 2, 0, []int{0, 1})
	assertBranch(t, level.Branches[1], confirmedStep("A", "C"), 1, 0, []int{2})

	next := query.GroupPathBranches(paths, level.Branches[0].PathIndexes, 1, query.PathFromStart)
	assertLevelAccounting(t, next, []int{0, 1}, len(paths))
	assertBranch(t, next.Branches[0], confirmedStep("B", "D"), 1, 0, []int{0})
	assertBranch(t, next.Branches[1], confirmedStep("B", "E"), 1, 0, []int{1})
}

func TestGroupPathBranchesGroupsReverseWithoutReversingSteps(t *testing.T) {
	paths := []query.SemanticPath{
		semanticPath(confirmedStep("A", "X"), confirmedStep("X", "Z")),
		semanticPath(confirmedStep("B", "X"), confirmedStep("X", "Z")),
		semanticPath(confirmedStep("C", "X"), confirmedStep("X", "Z")),
	}

	level := query.GroupPathBranches(paths, nil, 0, query.PathFromEnd)
	assertLevelAccounting(t, level, nil, len(paths))
	if len(level.Branches) != 1 {
		t.Fatalf("reverse level = %#v, want one converged branch", level)
	}
	assertBranch(t, level.Branches[0], confirmedStep("X", "Z"), 3, 0, []int{0, 1, 2})

	next := query.GroupPathBranches(paths, level.Branches[0].PathIndexes, 1, query.PathFromEnd)
	assertLevelAccounting(t, next, []int{0, 1, 2}, len(paths))
	for index, from := range []graph.SymbolRef{"A", "B", "C"} {
		assertBranch(t, next.Branches[index], confirmedStep(from, "X"), 1, 0, []int{index})
	}
}

func TestGroupPathBranchesAccountsForVariableLengthTerminals(t *testing.T) {
	paths := []query.SemanticPath{
		semanticPath(confirmedStep("X", "Z")),
		semanticPath(confirmedStep("A", "X"), confirmedStep("X", "Z")),
		semanticPath(confirmedStep("B", "X"), confirmedStep("X", "Z")),
	}
	first := query.GroupPathBranches(paths, nil, 0, query.PathFromEnd)
	next := query.GroupPathBranches(paths, first.Branches[0].PathIndexes, 1, query.PathFromEnd)

	assertLevelAccounting(t, next, []int{0, 1, 2}, len(paths))
	if !reflect.DeepEqual(next.TerminalPathIndexes, []int{0}) {
		t.Fatalf("terminal indexes = %v, want [0]", next.TerminalPathIndexes)
	}
	assertBranch(t, next.Branches[0], confirmedStep("A", "X"), 1, 0, []int{1})
	assertBranch(t, next.Branches[1], confirmedStep("B", "X"), 1, 0, []int{2})
}

func TestGroupPathBranchesKeepsStepAndWholePathCertaintyDistinct(t *testing.T) {
	confirmedAB := semanticStep("A", "B", graph.EdgeImplements, graph.RelationshipConfirmed)
	uncertainAB := semanticStep("A", "B", graph.EdgeImplements, graph.RelationshipUncertain)
	paths := []query.SemanticPath{
		semanticPath(confirmedAB, semanticStep("B", "Z", graph.EdgeCalls, graph.RelationshipUncertain)),
		semanticPath(confirmedAB, semanticStep("B", "Z", graph.EdgeReturns, graph.RelationshipConfirmed)),
		semanticPath(uncertainAB, semanticStep("B", "Z", graph.EdgeCalls, graph.RelationshipConfirmed)),
	}

	level := query.GroupPathBranches(paths, nil, 0, query.PathFromStart)
	assertLevelAccounting(t, level, nil, len(paths))
	if level.UncertainPathCount != 2 {
		t.Fatalf("level uncertain paths = %d, want 2", level.UncertainPathCount)
	}
	assertBranch(t, level.Branches[0], confirmedAB, 2, 1, []int{0, 1})
	assertBranch(t, level.Branches[1], uncertainAB, 1, 1, []int{2})
}

func TestGroupPathBranchesIsSemanticallyStableAcrossInputOrder(t *testing.T) {
	paths := []query.SemanticPath{
		semanticPath(confirmedStep("A", "B"), confirmedStep("B", "D")),
		semanticPath(confirmedStep("A", "B"), confirmedStep("B", "E")),
		semanticPath(confirmedStep("A", "C"), confirmedStep("C", "E")),
	}
	forward := query.GroupPathBranches(paths, nil, 0, query.PathFromStart)
	reversedPaths := slices.Clone(paths)
	slices.Reverse(reversedPaths)
	reversed := query.GroupPathBranches(reversedPaths, nil, 0, query.PathFromStart)

	if got, want := semanticLevelSummary(reversedPaths, reversed), semanticLevelSummary(paths, forward); !reflect.DeepEqual(got, want) {
		t.Fatalf("reordered semantic grouping differs:\n got: %#v\nwant: %#v", got, want)
	}
}

func TestGroupPathBranchesUsesSemanticStepOrdering(t *testing.T) {
	confirmedAB := semanticStep("A", "B", graph.EdgeCalls, graph.RelationshipConfirmed)
	uncertainAB := semanticStep("A", "B", graph.EdgeCalls, graph.RelationshipUncertain)
	confirmedAC := semanticStep("A", "C", graph.EdgeCalls, graph.RelationshipConfirmed)
	returnsAB := semanticStep("A", "B", graph.EdgeReturns, graph.RelationshipConfirmed)
	fromB := semanticStep("B", "A", graph.EdgeCalls, graph.RelationshipConfirmed)
	paths := []query.SemanticPath{
		semanticPath(fromB),
		semanticPath(returnsAB),
		semanticPath(uncertainAB),
		semanticPath(confirmedAC),
		semanticPath(confirmedAB),
	}

	level := query.GroupPathBranches(paths, nil, 0, query.PathFromStart)
	want := []query.SemanticStep{confirmedAB, uncertainAB, confirmedAC, returnsAB, fromB}
	for index, step := range want {
		if level.Branches[index].Step != step {
			t.Fatalf("branch %d step = %#v, want %#v; branches: %#v", index, level.Branches[index].Step, step, level.Branches)
		}
	}
}

func TestGroupPathBranchesNilEmptyAndExplicitEmptySelection(t *testing.T) {
	for _, level := range []query.PathBranchLevel{
		query.GroupPathBranches(nil, nil, 0, query.PathFromStart),
		query.GroupPathBranches([]query.SemanticPath{}, nil, 0, query.PathFromStart),
		query.GroupPathBranches([]query.SemanticPath{semanticPath(confirmedStep("A", "B"))}, []int{}, 0, query.PathFromStart),
	} {
		if !reflect.DeepEqual(level, query.PathBranchLevel{}) {
			t.Errorf("empty grouping = %#v, want zero value", level)
		}
	}
	if got := query.GroupPathBranches([]query.SemanticPath{semanticPath(confirmedStep("A", "B"))}, nil, 0, query.PathFromStart); got.PathCount != 1 {
		t.Fatalf("nil selector path count = %d, want 1", got.PathCount)
	}
}

func TestGroupPathBranchesReturnsDetachedSortedIndexes(t *testing.T) {
	paths := []query.SemanticPath{
		semanticPath(confirmedStep("A", "B")),
		semanticPath(confirmedStep("A", "C")),
		semanticPath(confirmedStep("A", "B")),
	}
	before := append([]query.SemanticPath(nil), paths...)
	selector := []int{2, 1, 0}
	level := query.GroupPathBranches(paths, selector, 0, query.PathFromStart)
	if !reflect.DeepEqual(level.Branches[0].PathIndexes, []int{0, 2}) || !reflect.DeepEqual(level.Branches[1].PathIndexes, []int{1}) {
		t.Fatalf("branch indexes are not sorted: %#v", level.Branches)
	}
	level.Branches[0].PathIndexes[0] = 99
	if !reflect.DeepEqual(selector, []int{2, 1, 0}) || !reflect.DeepEqual(level.Branches[1].PathIndexes, []int{1}) || !reflect.DeepEqual(paths, before) {
		t.Fatalf("mutating branch indexes changed other state: selector=%v branches=%#v paths=%#v", selector, level.Branches, paths)
	}
	again := query.GroupPathBranches(paths, selector, 0, query.PathFromStart)
	if !reflect.DeepEqual(again.Branches[0].PathIndexes, []int{0, 2}) {
		t.Fatalf("later call was corrupted: %#v", again)
	}

	terminal := query.GroupPathBranches(paths, selector, 1, query.PathFromStart)
	terminal.TerminalPathIndexes[0] = 99
	againTerminal := query.GroupPathBranches(paths, selector, 1, query.PathFromStart)
	if !reflect.DeepEqual(againTerminal.TerminalPathIndexes, []int{0, 1, 2}) {
		t.Fatalf("terminal indexes alias another result: %#v", againTerminal)
	}
}

func TestGroupPathBranchesRejectsInvalidProgrammerInput(t *testing.T) {
	paths := []query.SemanticPath{semanticPath(confirmedStep("A", "B"))}
	tests := []struct {
		name      string
		paths     []query.SemanticPath
		indexes   []int
		depth     int
		direction query.PathDirection
	}{
		{name: "negative depth", paths: paths, depth: -1, direction: query.PathFromStart},
		{name: "invalid direction", paths: paths, direction: query.PathDirection(99)},
		{name: "negative index", paths: paths, indexes: []int{-1}, direction: query.PathFromStart},
		{name: "large index", paths: paths, indexes: []int{1}, direction: query.PathFromStart},
		{name: "duplicate index", paths: paths, indexes: []int{0, 0}, direction: query.PathFromStart},
		{
			name:      "unknown certainty",
			paths:     []query.SemanticPath{semanticPath(semanticStep("A", "B", graph.EdgeCalls, graph.RelationshipCertaintyUnknown))},
			direction: query.PathFromStart,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Fatal("GroupPathBranches did not panic")
				}
			}()
			query.GroupPathBranches(test.paths, test.indexes, test.depth, test.direction)
		})
	}
}

func semanticPath(steps ...query.SemanticStep) query.SemanticPath {
	return query.SemanticPath{Steps: steps}
}

func confirmedStep(from, to graph.SymbolRef) query.SemanticStep {
	return semanticStep(from, to, graph.EdgeCalls, graph.RelationshipConfirmed)
}

func semanticStep(from, to graph.SymbolRef, kind graph.EdgeKind, certainty graph.RelationshipCertainty) query.SemanticStep {
	return query.SemanticStep{From: from, To: to, Kind: kind, Certainty: certainty}
}

func assertBranch(t *testing.T, got query.PathBranch, step query.SemanticStep, count, uncertain int, indexes []int) {
	t.Helper()
	if got.Step != step || got.PathCount != count || got.UncertainPathCount != uncertain || !reflect.DeepEqual(got.PathIndexes, indexes) {
		t.Fatalf("branch = %#v, want step %#v, count %d, uncertain %d, indexes %v", got, step, count, uncertain, indexes)
	}
}

func assertLevelAccounting(t *testing.T, level query.PathBranchLevel, selected []int, totalPaths int) {
	t.Helper()
	if selected == nil {
		selected = make([]int, totalPaths)
		for index := range selected {
			selected[index] = index
		}
	}
	seen := make(map[int]int, len(selected))
	for _, index := range level.TerminalPathIndexes {
		seen[index]++
	}
	accounted := len(level.TerminalPathIndexes)
	for _, branch := range level.Branches {
		if branch.PathCount != len(branch.PathIndexes) {
			t.Errorf("branch count %d != index count %d: %#v", branch.PathCount, len(branch.PathIndexes), branch)
		}
		accounted += branch.PathCount
		for _, index := range branch.PathIndexes {
			seen[index]++
		}
	}
	if level.PathCount != len(selected) || level.PathCount != accounted {
		t.Fatalf("level accounting = count %d, selected %d, accounted %d: %#v", level.PathCount, len(selected), accounted, level)
	}
	for _, index := range selected {
		if seen[index] != 1 {
			t.Errorf("selected index %d occurs %d times, want once", index, seen[index])
		}
	}
}

type levelSummary struct {
	pathCount      int
	uncertainCount int
	terminalPaths  []string
	branches       []branchSummary
}

type branchSummary struct {
	step           query.SemanticStep
	pathCount      int
	uncertainCount int
	paths          []string
}

func semanticLevelSummary(paths []query.SemanticPath, level query.PathBranchLevel) levelSummary {
	summary := levelSummary{pathCount: level.PathCount, uncertainCount: level.UncertainPathCount}
	for _, index := range level.TerminalPathIndexes {
		summary.terminalPaths = append(summary.terminalPaths, fmt.Sprint(paths[index]))
	}
	sort.Strings(summary.terminalPaths)
	for _, branch := range level.Branches {
		item := branchSummary{step: branch.Step, pathCount: branch.PathCount, uncertainCount: branch.UncertainPathCount}
		for _, index := range branch.PathIndexes {
			item.paths = append(item.paths, fmt.Sprint(paths[index]))
		}
		sort.Strings(item.paths)
		summary.branches = append(summary.branches, item)
	}
	return summary
}
