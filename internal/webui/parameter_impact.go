package webui

import (
	"encoding/json"
	"io"
	"net/http"

	"nocv/goanalyzer"
	"nocv/graph"
	"nocv/query"
)

type signatureImpactRequest struct {
	Callable   graph.SymbolRef        `json:"callable"`
	Parameters []parameterImpactInput `json:"parameters"`
	Results    []parameterImpactInput `json:"results"`
	Variadic   bool                   `json:"variadic"`
}

type parameterImpactInput struct {
	Type string `json:"type"`
}

type signatureImpact struct {
	Callable   graph.SymbolRef    `json:"callable"`
	Before     callableSignature  `json:"before"`
	After      callableSignature  `json:"after"`
	CallSites  []callSiteImpact   `json:"callSites"`
	Compiler   compilerImpact     `json:"compiler"`
	Contracts  []contractImpact   `json:"contracts"`
	Structural []structuralImpact `json:"structural"`
}

type callableSignature struct {
	Parameters []parameter `json:"parameters"`
	Results    []parameter `json:"results"`
	Variadic   bool        `json:"variadic"`
}

type parameter struct {
	Name string    `json:"name,omitempty"`
	Type goTypeRef `json:"type"`
}

type goTypeRef struct {
	Display string          `json:"display"`
	Symbol  graph.SymbolRef `json:"symbol,omitempty"`
}

type callSiteImpact struct {
	Caller        symbolSummary      `json:"caller"`
	Location      location           `json:"location"`
	Compatibility string             `json:"compatibility"`
	Problems      []signatureProblem `json:"problems"`
}

type signatureProblem struct {
	Kind          string    `json:"kind"`
	Argument      int       `json:"argument,omitempty"`
	Expected      goTypeRef `json:"expected"`
	Actual        goTypeRef `json:"actual"`
	ExpectedCount int       `json:"expectedCount,omitempty"`
	ActualCount   int       `json:"actualCount,omitempty"`
}

type contractImpact struct {
	Kind            string        `json:"kind"`
	Concrete        symbolSummary `json:"concrete"`
	Interface       symbolSummary `json:"interface"`
	ConcreteMethod  symbolSummary `json:"concreteMethod"`
	InterfaceMethod symbolSummary `json:"interfaceMethod"`
}

type structuralImpact struct {
	Kind         string        `json:"kind"`
	Type         symbolSummary `json:"type"`
	Exposure     string        `json:"exposure"`
	OriginMethod symbolSummary `json:"originMethod"`
}

type compilerImpact struct {
	AffectedPackages []graph.SymbolRef     `json:"affectedPackages"`
	Consequences     []compilerConsequence `json:"consequences"`
	BaselineStatus   string                `json:"baselineStatus"`
}

type compilerConsequence struct {
	Package        graph.SymbolRef `json:"package"`
	Symbol         *symbolSummary  `json:"symbol,omitempty"`
	Location       location        `json:"location"`
	Message        string          `json:"message"`
	Classification string          `json:"classification"`
}

func handleSignatureImpact(w http.ResponseWriter, r *http.Request, analysis *goanalyzer.Analysis) {
	if analysis == nil {
		writeError(w, http.StatusInternalServerError, "Go analysis is unavailable")
		return
	}
	var request signatureImpactRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON request: "+err.Error())
		return
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		writeError(w, http.StatusBadRequest, "request must contain one JSON object")
		return
	}
	if request.Callable == "" {
		writeError(w, http.StatusBadRequest, "callable is required")
		return
	}

	proposed := query.ProposedSignature{Variadic: request.Variadic}
	for _, parameter := range request.Parameters {
		proposed.Parameters = append(proposed.Parameters, query.ProposedParameter{TypeExpr: parameter.Type})
	}
	for _, result := range request.Results {
		proposed.Results = append(proposed.Results, query.ProposedResult{TypeExpr: result.Type})
	}
	impact, err := analysis.AnalyzeSignatureChange(request.Callable, proposed)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, presentSignatureImpact(impact))
}

func presentSignatureImpact(source goanalyzer.SignatureChangeImpact) signatureImpact {
	result := signatureImpact{
		Callable:  source.Callable,
		Before:    presentCallableSignature(source.Before),
		After:     presentCallableSignature(source.After),
		CallSites: make([]callSiteImpact, 0, len(source.CallSites)),
		Compiler: compilerImpact{
			AffectedPackages: append([]graph.SymbolRef(nil), source.Compiler.AffectedPackages...),
			Consequences:     make([]compilerConsequence, 0, len(source.Compiler.Consequences)),
			BaselineStatus:   source.Compiler.BaselineStatus.String(),
		},
		Contracts:  make([]contractImpact, 0, len(source.Contracts)),
		Structural: make([]structuralImpact, 0, len(source.Structural)),
	}
	for _, consequence := range source.Compiler.Consequences {
		presented := compilerConsequence{
			Package: consequence.Package, Location: location{File: consequence.Location.File, Offset: consequence.Location.Offset},
			Message: consequence.Message, Classification: consequence.Classification.String(),
		}
		if consequence.Symbol != nil {
			symbol := presentSymbolSummary(*consequence.Symbol)
			presented.Symbol = &symbol
		}
		result.Compiler.Consequences = append(result.Compiler.Consequences, presented)
	}
	for _, site := range source.CallSites {
		presented := callSiteImpact{
			Caller:        presentSymbolSummary(site.Caller),
			Location:      location{File: site.Location.File, Offset: site.Location.Offset},
			Compatibility: site.Compatibility.String(),
			Problems:      make([]signatureProblem, 0, len(site.Problems)),
		}
		for _, problem := range site.Problems {
			presented.Problems = append(presented.Problems, signatureProblem{
				Kind:          problem.Kind.String(),
				Argument:      problem.Argument,
				Expected:      presentGoTypeRef(problem.Expected),
				Actual:        presentGoTypeRef(problem.Actual),
				ExpectedCount: problem.ExpectedCount,
				ActualCount:   problem.ActualCount,
			})
		}
		result.CallSites = append(result.CallSites, presented)
	}
	for _, contract := range source.Contracts {
		result.Contracts = append(result.Contracts, contractImpact{
			Kind:            contract.Kind.String(),
			Concrete:        presentSymbolSummary(contract.Concrete),
			Interface:       presentSymbolSummary(contract.Interface),
			ConcreteMethod:  presentSymbolSummary(contract.ConcreteMethod),
			InterfaceMethod: presentSymbolSummary(contract.InterfaceMethod),
		})
	}
	for _, structural := range source.Structural {
		result.Structural = append(result.Structural, structuralImpact{
			Kind:         structural.Kind.String(),
			Type:         presentSymbolSummary(structural.Type),
			Exposure:     structural.Exposure.String(),
			OriginMethod: presentSymbolSummary(structural.OriginMethod),
		})
	}
	return result
}

func presentCallableSignature(source query.CallableSignature) callableSignature {
	result := callableSignature{
		Parameters: make([]parameter, 0, len(source.Parameters)),
		Results:    make([]parameter, 0, len(source.Results)),
		Variadic:   source.Variadic,
	}
	for _, sourceParameter := range source.Parameters {
		result.Parameters = append(result.Parameters, parameter{Name: sourceParameter.Name, Type: presentGoTypeRef(sourceParameter.Type)})
	}
	for _, sourceResult := range source.Results {
		result.Results = append(result.Results, parameter{Name: sourceResult.Name, Type: presentGoTypeRef(sourceResult.Type)})
	}
	return result
}

func presentGoTypeRef(source query.GoTypeRef) goTypeRef {
	return goTypeRef{Display: source.Display, Symbol: source.Symbol}
}
