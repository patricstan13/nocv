package query

import "nocv/graph"

// CallableSignature is a detached presentation of a callable's parameters and
// results. The receiver and type parameters are deliberately outside the
// hypothetical signature-change model.
type CallableSignature struct {
	Parameters []Parameter
	Results    []Result
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

// Result is one ordered callable result. Name is descriptive only and does not
// participate in signature identity.
type Result struct {
	Name string
	Type GoTypeRef
}

// ProposedResult is a Go type expression to resolve in the callable's
// declaration-file context. Name is optional and semantically irrelevant.
type ProposedResult struct {
	Name     string
	TypeExpr string
}

// ProposedSignature is the public, compiler-independent input for a
// hypothetical callable signature.
type ProposedSignature struct {
	Parameters []ProposedParameter
	Results    []ProposedResult
	Variadic   bool
}

// GoTypeRef is a detached presentation of a Go type. Symbol is populated only
// when the type directly refers to a declaration represented by the graph.
type GoTypeRef struct {
	Display string
	Symbol  graph.SymbolRef
}

// ParameterChangeImpact remains as the compatibility model for the
// impact-params CLI. New product surfaces use goanalyzer.SignatureChangeImpact.
type ParameterChangeImpact struct {
	Callable   graph.SymbolRef
	Before     CallableSignature
	After      CallableSignature
	CallSites  []CallSiteImpact
	Contracts  []ContractImpact
	Structural []StructuralImpact
}

// StructuralImpact describes a modeled type whose effective method surface is
// changed because the selected concrete method is promoted through embedding.
type StructuralImpact struct {
	Kind         StructuralImpactKind
	Type         SymbolSummary
	Exposure     MethodExposure
	OriginMethod SymbolSummary
}

// MethodExposure describes whether a promoted method is visible on T itself
// or only on *T. Value implies visibility on T; pointer accessibility then
// follows Go's method-set rules without requiring a second impact row.
type MethodExposure uint8

const (
	MethodExposureUnknown MethodExposure = iota
	MethodExposureValue
	MethodExposurePointerOnly
)

func (e MethodExposure) String() string {
	switch e {
	case MethodExposureValue:
		return "value"
	case MethodExposurePointerOnly:
		return "pointer only"
	default:
		return "unknown"
	}
}

// StructuralImpactKind identifies a deterministic structural consequence.
type StructuralImpactKind uint8

const (
	StructuralImpactUnknown StructuralImpactKind = iota
	StructuralPromotedMethodChanged
)

func (k StructuralImpactKind) String() string {
	switch k {
	case StructuralPromotedMethodChanged:
		return "promoted method changed"
	default:
		return "unknown structural impact"
	}
}

// ContractImpact describes one existing concrete-type/interface contract that
// the hypothetical parameter signature would invalidate.
type ContractImpact struct {
	Kind            ContractImpactKind
	Concrete        SymbolSummary
	Interface       SymbolSummary
	ConcreteMethod  SymbolSummary
	InterfaceMethod SymbolSummary
}

// ContractImpactKind identifies a deterministic contract consequence.
type ContractImpactKind uint8

const (
	ContractImpactUnknown ContractImpactKind = iota
	ContractImplementationLost
)

func (k ContractImpactKind) String() string {
	switch k {
	case ContractImplementationLost:
		return "lost implementation"
	default:
		return "unknown contract impact"
	}
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
