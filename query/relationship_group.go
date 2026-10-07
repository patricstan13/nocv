package query

import (
	"sort"

	"nocv/graph"
)

// RelationshipGroup summarizes relationships with the same kind and certainty.
// Example is a detached, deterministic representative of the group.
type RelationshipGroup struct {
	Kind      graph.EdgeKind
	Certainty graph.RelationshipCertainty
	Count     int
	Example   Relationship
}

// GroupRelationships projects exact relationships into groups by kind and
// certainty. Each relationship contributes one to Count, regardless of how
// many evidence locations it contains.
func GroupRelationships(relationships []Relationship) []RelationshipGroup {
	if len(relationships) == 0 {
		return nil
	}

	type groupKey struct {
		kind      graph.EdgeKind
		certainty graph.RelationshipCertainty
	}

	groupsByKey := make(map[groupKey]RelationshipGroup)
	for _, relationship := range relationships {
		key := groupKey{kind: relationship.Kind, certainty: relationship.Certainty}
		group, exists := groupsByKey[key]
		group.Count++
		if !exists || relationshipExampleLess(relationship, group.Example) {
			group.Kind = relationship.Kind
			group.Certainty = relationship.Certainty
			group.Example = copyRelationship(relationship)
		}
		groupsByKey[key] = group
	}

	groups := make([]RelationshipGroup, 0, len(groupsByKey))
	for _, group := range groupsByKey {
		groups = append(groups, group)
	}
	sort.Slice(groups, func(i, j int) bool {
		leftKind, rightKind := groups[i].Kind.String(), groups[j].Kind.String()
		if leftKind != rightKind {
			return leftKind < rightKind
		}
		if groups[i].Kind != groups[j].Kind {
			return groups[i].Kind < groups[j].Kind
		}
		return relationshipCertaintyRank(groups[i].Certainty) < relationshipCertaintyRank(groups[j].Certainty)
	})
	return groups
}

func relationshipCertaintyRank(certainty graph.RelationshipCertainty) int {
	switch certainty {
	case graph.RelationshipConfirmed:
		return 0
	case graph.RelationshipUncertain:
		return 1
	default:
		return 2 + int(certainty)
	}
}

func relationshipExampleLess(left, right Relationship) bool {
	if semanticRelationshipLess(left, right) {
		return true
	}
	if semanticRelationshipLess(right, left) {
		return false
	}

	shared := min(len(left.Evidence), len(right.Evidence))
	for index := 0; index < shared; index++ {
		if locationLess(left.Evidence[index], right.Evidence[index]) {
			return true
		}
		if locationLess(right.Evidence[index], left.Evidence[index]) {
			return false
		}
	}
	return len(left.Evidence) < len(right.Evidence)
}

func locationLess(left, right graph.Location) bool {
	if left.File != right.File {
		return left.File < right.File
	}
	if left.Offset != right.Offset {
		return left.Offset < right.Offset
	}
	if left.Line != right.Line {
		return left.Line < right.Line
	}
	return left.Column < right.Column
}
