package query

import (
	"sort"

	"nocv/graph"
)

// PackagePathBranchLevel is one progressively requested level of exact
// package dependency path groups. Indexes refer to the caller's paths slice.
type PackagePathBranchLevel struct {
	PathCount           int
	UncertainPathCount  int
	TerminalPathIndexes []int
	Branches            []PackagePathBranch
}

// PackagePathBranch groups selected exact paths by one projected package
// dependency. Aggregated evidence is retained but is not branch identity.
type PackagePathBranch struct {
	Step               PackageDependency
	PathCount          int
	UncertainPathCount int
	PathIndexes        []int
}

type packagePathBranchKey struct {
	from      graph.SymbolRef
	to        graph.SymbolRef
	certainty graph.RelationshipCertainty
}

// GroupPackagePathBranches partitions selected exact package paths by the
// source-side step at depth. A nil pathIndexes slice selects every path.
func GroupPackagePathBranches(
	paths []PackageDependencyPath,
	pathIndexes []int,
	depth int,
) PackagePathBranchLevel {
	if depth < 0 {
		panic("query: package path branch depth must be non-negative")
	}

	selected := selectedPathIndexes(len(paths), pathIndexes)
	if len(selected) == 0 {
		return PackagePathBranchLevel{}
	}

	level := PackagePathBranchLevel{PathCount: len(selected)}
	branchesByStep := make(map[packagePathBranchKey]*PackagePathBranch)
	for _, pathIndex := range selected {
		path := paths[pathIndex]
		uncertain := packageDependencyPathIsUncertain(path)
		if uncertain {
			level.UncertainPathCount++
		}

		if depth >= len(path.Steps) {
			level.TerminalPathIndexes = append(level.TerminalPathIndexes, pathIndex)
			continue
		}

		step := path.Steps[depth]
		key := packagePathBranchKey{from: step.From, to: step.To, certainty: step.Certainty}
		branch := branchesByStep[key]
		if branch == nil {
			branch = &PackagePathBranch{Step: copyPackageDependency(step)}
			branchesByStep[key] = branch
		}
		branch.PathCount++
		if uncertain {
			branch.UncertainPathCount++
		}
		branch.PathIndexes = append(branch.PathIndexes, pathIndex)
	}

	if len(branchesByStep) > 0 {
		level.Branches = make([]PackagePathBranch, 0, len(branchesByStep))
		for _, branch := range branchesByStep {
			level.Branches = append(level.Branches, *branch)
		}
	}
	sort.Slice(level.Branches, func(i, j int) bool {
		return packagePathBranchStepLess(level.Branches[i].Step, level.Branches[j].Step)
	})
	return level
}

func packageDependencyPathIsUncertain(path PackageDependencyPath) bool {
	uncertain := false
	for _, step := range path.Steps {
		switch step.Certainty {
		case graph.RelationshipConfirmed:
		case graph.RelationshipUncertain:
			uncertain = true
		default:
			panic("query: package dependency path contains invalid relationship certainty")
		}
	}
	return uncertain
}

func packagePathBranchStepLess(left, right PackageDependency) bool {
	if left.From != right.From {
		return left.From < right.From
	}
	if left.To != right.To {
		return left.To < right.To
	}
	return relationshipCertaintyRank(left.Certainty) < relationshipCertaintyRank(right.Certainty)
}
