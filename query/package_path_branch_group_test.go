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

func TestGroupPackagePathBranchesGroupsForwardAndExpandsSubset(t *testing.T) {
	paths := []query.PackageDependencyPath{
		packagePath(packageStep("A", "B", graph.RelationshipConfirmed), packageStep("B", "D", graph.RelationshipConfirmed)),
		packagePath(packageStep("A", "B", graph.RelationshipConfirmed), packageStep("B", "E", graph.RelationshipConfirmed)),
		packagePath(packageStep("A", "C", graph.RelationshipConfirmed), packageStep("C", "E", graph.RelationshipConfirmed)),
	}

	level := query.GroupPackagePathBranches(paths, nil, 0)
	assertPackageLevelAccounting(t, level, nil, len(paths))
	assertPackageBranch(t, level.Branches[0], "A", "B", graph.RelationshipConfirmed, 2, 0, []int{0, 1})
	assertPackageBranch(t, level.Branches[1], "A", "C", graph.RelationshipConfirmed, 1, 0, []int{2})

	next := query.GroupPackagePathBranches(paths, level.Branches[0].PathIndexes, 1)
	assertPackageLevelAccounting(t, next, []int{0, 1}, len(paths))
	assertPackageBranch(t, next.Branches[0], "B", "D", graph.RelationshipConfirmed, 1, 0, []int{0})
	assertPackageBranch(t, next.Branches[1], "B", "E", graph.RelationshipConfirmed, 1, 0, []int{1})
}

func TestGroupPackagePathBranchesAccountsForDirectPathTerminal(t *testing.T) {
	paths := []query.PackageDependencyPath{
		packagePath(packageStep("A", "C", graph.RelationshipConfirmed)),
		packagePath(packageStep("A", "B", graph.RelationshipConfirmed), packageStep("B", "C", graph.RelationshipConfirmed)),
	}
	first := query.GroupPackagePathBranches(paths, nil, 0)
	assertPackageLevelAccounting(t, first, nil, len(paths))

	direct := first.Branches[1]
	assertPackageBranch(t, direct, "A", "C", graph.RelationshipConfirmed, 1, 0, []int{0})
	next := query.GroupPackagePathBranches(paths, direct.PathIndexes, 1)
	assertPackageLevelAccounting(t, next, []int{0}, len(paths))
	if !reflect.DeepEqual(next.TerminalPathIndexes, []int{0}) || next.Branches != nil {
		t.Fatalf("direct path expansion = %#v, want terminal [0] and no branches", next)
	}
}

func TestGroupPackagePathBranchesKeepsStepAndWholePathCertaintyDistinct(t *testing.T) {
	paths := []query.PackageDependencyPath{
		packagePath(packageStep("A", "B", graph.RelationshipConfirmed), packageStep("B", "C", graph.RelationshipUncertain)),
		packagePath(packageStep("A", "B", graph.RelationshipConfirmed), packageStep("B", "D", graph.RelationshipConfirmed)),
		packagePath(packageStep("A", "B", graph.RelationshipUncertain), packageStep("B", "E", graph.RelationshipConfirmed)),
	}

	level := query.GroupPackagePathBranches(paths, nil, 0)
	assertPackageLevelAccounting(t, level, nil, len(paths))
	if level.UncertainPathCount != 2 {
		t.Fatalf("level uncertain paths = %d, want 2", level.UncertainPathCount)
	}
	assertPackageBranch(t, level.Branches[0], "A", "B", graph.RelationshipConfirmed, 2, 1, []int{0, 1})
	assertPackageBranch(t, level.Branches[1], "A", "B", graph.RelationshipUncertain, 1, 1, []int{2})
}

func TestGroupPackagePathBranchesExcludesEvidenceFromIdentityAndDetachesIt(t *testing.T) {
	firstEvidence := packageEvidence("A::One", "B::One", graph.EdgeCalls, 1)
	secondEvidence := packageEvidence("A::Two", "B::Two", graph.EdgeReturns, 2)
	paths := []query.PackageDependencyPath{
		packagePath(packageStepWithEvidence("A", "B", graph.RelationshipConfirmed, firstEvidence)),
		packagePath(packageStepWithEvidence("A", "B", graph.RelationshipConfirmed, secondEvidence)),
		packagePath(packageStepWithEvidence("A", "C", graph.RelationshipConfirmed, packageEvidence("A::Three", "C::One", graph.EdgeCalls, 3))),
	}

	level := query.GroupPackagePathBranches(paths, nil, 0)
	assertPackageLevelAccounting(t, level, nil, len(paths))
	assertPackageBranch(t, level.Branches[0], "A", "B", graph.RelationshipConfirmed, 2, 0, []int{0, 1})
	if !reflect.DeepEqual(level.Branches[0].Step.Evidence, []query.Relationship{firstEvidence}) {
		t.Fatalf("branch evidence = %#v, want detached first projected step evidence", level.Branches[0].Step.Evidence)
	}

	level.Branches[0].Step.Evidence[0].From = "mutated"
	level.Branches[0].Step.Evidence[0].Evidence[0].Offset = 99
	if paths[0].Steps[0].Evidence[0].From != "A::One" || paths[0].Steps[0].Evidence[0].Evidence[0].Offset != 1 {
		t.Fatalf("branch evidence aliases exact input: %#v", paths[0])
	}
	if level.Branches[1].Step.Evidence[0].From != "A::Three" {
		t.Fatalf("branch evidence aliases sibling branch: %#v", level.Branches)
	}
}

func TestGroupPackagePathBranchesIsStableAcrossInputOrder(t *testing.T) {
	paths := []query.PackageDependencyPath{
		packagePath(packageStep("A", "B", graph.RelationshipConfirmed), packageStep("B", "D", graph.RelationshipConfirmed)),
		packagePath(packageStep("A", "B", graph.RelationshipConfirmed), packageStep("B", "E", graph.RelationshipUncertain)),
		packagePath(packageStep("A", "C", graph.RelationshipConfirmed), packageStep("C", "E", graph.RelationshipConfirmed)),
	}
	level := query.GroupPackagePathBranches(paths, nil, 0)
	reorderedPaths := slices.Clone(paths)
	slices.Reverse(reorderedPaths)
	reordered := query.GroupPackagePathBranches(reorderedPaths, nil, 0)

	if got, want := packageLevelSummary(reorderedPaths, reordered), packageLevelSummary(paths, level); !reflect.DeepEqual(got, want) {
		t.Fatalf("reordered package grouping differs:\n got: %#v\nwant: %#v", got, want)
	}
}

func TestGroupPackagePathBranchesUsesPackageStepOrdering(t *testing.T) {
	paths := []query.PackageDependencyPath{
		packagePath(packageStep("B", "A", graph.RelationshipConfirmed)),
		packagePath(packageStep("A", "C", graph.RelationshipConfirmed)),
		packagePath(packageStep("A", "B", graph.RelationshipUncertain)),
		packagePath(packageStep("A", "B", graph.RelationshipConfirmed)),
	}

	level := query.GroupPackagePathBranches(paths, nil, 0)
	want := []struct {
		from, to  graph.SymbolRef
		certainty graph.RelationshipCertainty
	}{
		{from: "A", to: "B", certainty: graph.RelationshipConfirmed},
		{from: "A", to: "B", certainty: graph.RelationshipUncertain},
		{from: "A", to: "C", certainty: graph.RelationshipConfirmed},
		{from: "B", to: "A", certainty: graph.RelationshipConfirmed},
	}
	for index, expected := range want {
		step := level.Branches[index].Step
		if step.From != expected.from || step.To != expected.to || step.Certainty != expected.certainty {
			t.Fatalf("branch %d step = %#v, want %s -> %s certainty %s", index, step, expected.from, expected.to, expected.certainty)
		}
	}
}

func TestGroupPackagePathBranchesNilEmptyAndExplicitEmptySelection(t *testing.T) {
	for _, level := range []query.PackagePathBranchLevel{
		query.GroupPackagePathBranches(nil, nil, 0),
		query.GroupPackagePathBranches([]query.PackageDependencyPath{}, nil, 0),
		query.GroupPackagePathBranches([]query.PackageDependencyPath{packagePath(packageStep("A", "B", graph.RelationshipConfirmed))}, []int{}, 0),
	} {
		if !reflect.DeepEqual(level, query.PackagePathBranchLevel{}) {
			t.Errorf("empty grouping = %#v, want zero value", level)
		}
	}
	if got := query.GroupPackagePathBranches([]query.PackageDependencyPath{packagePath(packageStep("A", "B", graph.RelationshipConfirmed))}, nil, 0); got.PathCount != 1 {
		t.Fatalf("nil selector path count = %d, want 1", got.PathCount)
	}
}

func TestGroupPackagePathBranchesReturnsDetachedSortedIndexes(t *testing.T) {
	paths := []query.PackageDependencyPath{
		packagePath(packageStep("A", "B", graph.RelationshipConfirmed)),
		packagePath(packageStep("A", "C", graph.RelationshipConfirmed)),
		packagePath(packageStep("A", "B", graph.RelationshipConfirmed)),
	}
	selector := []int{2, 1, 0}
	level := query.GroupPackagePathBranches(paths, selector, 0)
	if !reflect.DeepEqual(level.Branches[0].PathIndexes, []int{0, 2}) || !reflect.DeepEqual(level.Branches[1].PathIndexes, []int{1}) {
		t.Fatalf("branch indexes are not sorted: %#v", level.Branches)
	}
	level.Branches[0].PathIndexes[0] = 99
	if !reflect.DeepEqual(selector, []int{2, 1, 0}) || !reflect.DeepEqual(level.Branches[1].PathIndexes, []int{1}) {
		t.Fatalf("mutating branch indexes changed selector or sibling: selector=%v branches=%#v", selector, level.Branches)
	}
	again := query.GroupPackagePathBranches(paths, selector, 0)
	if !reflect.DeepEqual(again.Branches[0].PathIndexes, []int{0, 2}) {
		t.Fatalf("later call was corrupted: %#v", again)
	}

	terminal := query.GroupPackagePathBranches(paths, selector, 1)
	terminal.TerminalPathIndexes[0] = 99
	againTerminal := query.GroupPackagePathBranches(paths, selector, 1)
	if !reflect.DeepEqual(againTerminal.TerminalPathIndexes, []int{0, 1, 2}) {
		t.Fatalf("terminal indexes alias another result: %#v", againTerminal)
	}
}

func TestGroupPackagePathBranchesRejectsInvalidProgrammerInput(t *testing.T) {
	paths := []query.PackageDependencyPath{packagePath(packageStep("A", "B", graph.RelationshipConfirmed))}
	tests := []struct {
		name    string
		paths   []query.PackageDependencyPath
		indexes []int
		depth   int
	}{
		{name: "negative depth", paths: paths, depth: -1},
		{name: "negative index", paths: paths, indexes: []int{-1}},
		{name: "large index", paths: paths, indexes: []int{1}},
		{name: "duplicate index", paths: paths, indexes: []int{0, 0}},
		{
			name:  "unknown certainty",
			paths: []query.PackageDependencyPath{packagePath(packageStep("A", "B", graph.RelationshipCertaintyUnknown))},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Fatal("GroupPackagePathBranches did not panic")
				}
			}()
			query.GroupPackagePathBranches(test.paths, test.indexes, test.depth)
		})
	}
}

func TestGroupPackagePathBranchesCollapsesCaddyShapedPaths(t *testing.T) {
	paths := []query.PackageDependencyPath{
		packagePath(packageStep("source", "pki", graph.RelationshipConfirmed), packageStep("pki", "one", graph.RelationshipUncertain), packageStep("one", "target", graph.RelationshipConfirmed)),
		packagePath(packageStep("source", "pki", graph.RelationshipConfirmed), packageStep("pki", "two", graph.RelationshipConfirmed), packageStep("two", "target", graph.RelationshipConfirmed)),
		packagePath(packageStep("source", "tls", graph.RelationshipConfirmed), packageStep("tls", "target", graph.RelationshipUncertain)),
		packagePath(packageStep("source", "config", graph.RelationshipConfirmed), packageStep("config", "target", graph.RelationshipConfirmed)),
		packagePath(packageStep("source", "target", graph.RelationshipConfirmed)),
	}

	level := query.GroupPackagePathBranches(paths, nil, 0)
	assertPackageLevelAccounting(t, level, nil, len(paths))
	if level.PathCount != 5 || len(level.Branches) != 4 || level.UncertainPathCount != 2 {
		t.Fatalf("Caddy-shaped grouping = %#v, want five paths, four families, two uncertain", level)
	}
}

func packagePath(steps ...query.PackageDependency) query.PackageDependencyPath {
	packages := make([]graph.SymbolRef, 0, len(steps)+1)
	if len(steps) > 0 {
		packages = append(packages, steps[0].From)
		for _, step := range steps {
			packages = append(packages, step.To)
		}
	}
	return query.PackageDependencyPath{Packages: packages, Steps: steps}
}

func packageStep(from, to graph.SymbolRef, certainty graph.RelationshipCertainty) query.PackageDependency {
	return query.PackageDependency{From: from, To: to, Certainty: certainty}
}

func packageStepWithEvidence(from, to graph.SymbolRef, certainty graph.RelationshipCertainty, evidence ...query.Relationship) query.PackageDependency {
	return query.PackageDependency{From: from, To: to, Certainty: certainty, Evidence: evidence}
}

func packageEvidence(from, to graph.SymbolRef, kind graph.EdgeKind, offset int) query.Relationship {
	return query.Relationship{
		From: from, To: to, Kind: kind, Certainty: graph.RelationshipConfirmed,
		Evidence: []graph.Location{{File: "paths.go", Offset: offset}},
	}
}

func assertPackageBranch(
	t *testing.T,
	got query.PackagePathBranch,
	from, to graph.SymbolRef,
	certainty graph.RelationshipCertainty,
	count, uncertain int,
	indexes []int,
) {
	t.Helper()
	if got.Step.From != from || got.Step.To != to || got.Step.Certainty != certainty ||
		got.PathCount != count || got.UncertainPathCount != uncertain || !reflect.DeepEqual(got.PathIndexes, indexes) {
		t.Fatalf("branch = %#v, want %s -> %s certainty %s, count %d, uncertain %d, indexes %v", got, from, to, certainty, count, uncertain, indexes)
	}
}

func assertPackageLevelAccounting(t *testing.T, level query.PackagePathBranchLevel, selected []int, totalPaths int) {
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

type packageLevelSemanticSummary struct {
	pathCount      int
	uncertainCount int
	terminalPaths  []string
	branches       []packageBranchSemanticSummary
}

type packageBranchSemanticSummary struct {
	from           graph.SymbolRef
	to             graph.SymbolRef
	certainty      graph.RelationshipCertainty
	pathCount      int
	uncertainCount int
	paths          []string
}

func packageLevelSummary(paths []query.PackageDependencyPath, level query.PackagePathBranchLevel) packageLevelSemanticSummary {
	summary := packageLevelSemanticSummary{pathCount: level.PathCount, uncertainCount: level.UncertainPathCount}
	for _, index := range level.TerminalPathIndexes {
		summary.terminalPaths = append(summary.terminalPaths, fmt.Sprint(paths[index].Packages))
	}
	sort.Strings(summary.terminalPaths)
	for _, branch := range level.Branches {
		item := packageBranchSemanticSummary{
			from: branch.Step.From, to: branch.Step.To, certainty: branch.Step.Certainty,
			pathCount: branch.PathCount, uncertainCount: branch.UncertainPathCount,
		}
		for _, index := range branch.PathIndexes {
			item.paths = append(item.paths, fmt.Sprint(paths[index].Packages))
		}
		sort.Strings(item.paths)
		summary.branches = append(summary.branches, item)
	}
	return summary
}
