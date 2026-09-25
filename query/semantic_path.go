package query

import (
	"strconv"
	"strings"

	"nocv/graph"
)

// SemanticPath is one ordered semantic dependency explanation.
type SemanticPath struct {
	Steps []SemanticStep
}

// SemanticStep is one semantic relationship in ordinary dependency direction.
type SemanticStep struct {
	From graph.SymbolID
	To   graph.SymbolID
	Kind graph.EdgeKind
}

func semanticPathLess(left, right SemanticPath) bool {
	if len(left.Steps) != len(right.Steps) {
		return len(left.Steps) < len(right.Steps)
	}
	for index := range left.Steps {
		leftStep := left.Steps[index]
		rightStep := right.Steps[index]
		if leftStep.From != rightStep.From {
			return leftStep.From < rightStep.From
		}
		if leftStep.Kind != rightStep.Kind {
			return leftStep.Kind < rightStep.Kind
		}
		if leftStep.To != rightStep.To {
			return leftStep.To < rightStep.To
		}
	}
	return false
}

func semanticPathKey(steps []SemanticStep) string {
	var key strings.Builder
	for _, step := range steps {
		writeSemanticPathID(&key, step.From)
		key.WriteByte('/')
		key.WriteString(strconv.FormatUint(uint64(step.Kind), 10))
		key.WriteByte('/')
		writeSemanticPathID(&key, step.To)
		key.WriteByte(';')
	}
	return key.String()
}

func writeSemanticPathID(builder *strings.Builder, id graph.SymbolID) {
	value := string(id)
	builder.WriteString(strconv.Itoa(len(value)))
	builder.WriteByte(':')
	builder.WriteString(value)
}
