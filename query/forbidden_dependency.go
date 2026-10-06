package query

import "nocv/graph"

// ForbiddenDependencyOutcome describes what the available semantic paths
// establish about one forbidden package dependency.
type ForbiddenDependencyOutcome uint8

const (
	ForbiddenDependencyNoObservedPath ForbiddenDependencyOutcome = iota
	ForbiddenDependencyPotentialViolation
	ForbiddenDependencyConfirmedViolation
)

func (outcome ForbiddenDependencyOutcome) String() string {
	switch outcome {
	case ForbiddenDependencyPotentialViolation:
		return "potential violation"
	case ForbiddenDependencyConfirmedViolation:
		return "confirmed violation"
	default:
		return "no observed violation"
	}
}

// PackageDependencyViolation is one forbidden semantic package dependency
// together with every package route and its hop-level boundary evidence.
type PackageDependencyViolation struct {
	From    graph.SymbolRef
	To      graph.SymbolRef
	Outcome ForbiddenDependencyOutcome
	Paths   []PackageDependencyPath
}

// CheckForbiddenPackageDependency reports whether PackageDependencyPaths
// contains at least one semantic dependency route from -> to.
func CheckForbiddenPackageDependency(
	g *graph.Graph,
	from graph.SymbolRef,
	to graph.SymbolRef,
) (PackageDependencyViolation, bool) {
	paths := PackageDependencyPaths(g, from, to)
	if len(paths) == 0 {
		return PackageDependencyViolation{}, false
	}
	outcome := ForbiddenDependencyPotentialViolation
	for _, path := range paths {
		if packageDependencyPathIsConfirmed(path) {
			outcome = ForbiddenDependencyConfirmedViolation
			break
		}
	}
	return PackageDependencyViolation{From: from, To: to, Outcome: outcome, Paths: paths}, true
}

func packageDependencyPathIsConfirmed(path PackageDependencyPath) bool {
	for _, step := range path.Steps {
		if step.Certainty != graph.RelationshipConfirmed {
			return false
		}
	}
	return len(path.Steps) > 0
}
