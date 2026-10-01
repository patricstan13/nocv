package query

import "nocv/graph"

// CallableSignature describes only a callable's parameters. Results and the
// receiver are deliberately outside the first call-site impact model.
type CallableSignature struct {
	Parameters []Parameter
	Variadic   bool
}

// Parameter is one ordered callable parameter. Name is descriptive only and
// does not participate in compatibility.
type Parameter struct {
	Name string
	Type GoTypeRef
}

// ProposedParameter is a Go type expression to resolve in the callable's
// declaration-file context. Name is optional and semantically irrelevant.
type ProposedParameter struct {
	Name     string
	TypeExpr string
}

// ProposedSignature is the public, compiler-independent input for a
// hypothetical parameter signature.
type ProposedSignature struct {
	Parameters []ProposedParameter
	Variadic   bool
}

// GoTypeRef is a detached presentation of a Go type. Symbol is populated only
// when the type directly refers to a declaration represented by the graph.
type GoTypeRef struct {
	Display string
	Symbol  graph.SymbolRef
}

// ParameterChangeImpact reports every known direct source call to Callable.
type ParameterChangeImpact struct {
	Callable  graph.SymbolRef
	Before    CallableSignature
	After     CallableSignature
	CallSites []CallSiteImpact
}

// CallSiteImpact describes compatibility for one concrete call expression.
type CallSiteImpact struct {
	Caller        SymbolSummary
	Location      graph.Location
	Compatibility Compatibility
	Problems      []SignatureProblem
}

// Compatibility is the compiler-backed compatibility state of a call site.
type Compatibility uint8

const (
	CompatibilityUnknown Compatibility = iota
	CompatibilityCompatible
	CompatibilityIncompatible
)

func (c Compatibility) String() string {
	switch c {
	case CompatibilityCompatible:
		return "compatible"
	case CompatibilityIncompatible:
		return "incompatible"
	case CompatibilityUnknown:
		return "unknown"
	default:
		return "unknown"
	}
}

// SignatureProblemKind identifies one concrete reason a call is not known to
// be compatible.
type SignatureProblemKind uint8

const (
	ProblemArgumentCount SignatureProblemKind = iota
	ProblemArgumentType
	ProblemVariadic
	ProblemUnknown
)

func (k SignatureProblemKind) String() string {
	switch k {
	case ProblemArgumentCount:
		return "argument count"
	case ProblemArgumentType:
		return "argument type"
	case ProblemVariadic:
		return "variadic call"
	case ProblemUnknown:
		return "unknown"
	default:
		return "unknown"
	}
}

// SignatureProblem is structured evidence for incompatibility or uncertainty.
// Argument is one-based for human-facing output; zero means no single argument
// applies, as with an argument-count mismatch.
type SignatureProblem struct {
	Kind          SignatureProblemKind
	Argument      int
	Expected      GoTypeRef
	Actual        GoTypeRef
	ExpectedCount int
	ActualCount   int
}
