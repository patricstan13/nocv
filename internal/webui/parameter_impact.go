package webui

import (
	"encoding/json"
	"io"
	"net/http"

	"nocv/goanalyzer"
	"nocv/graph"
	"nocv/query"
)

type parameterImpactRequest struct {
	Callable   graph.SymbolRef        `json:"callable"`
	Parameters []parameterImpactInput `json:"parameters"`
	Variadic   bool                   `json:"variadic"`
}

type parameterImpactInput struct {
	Type string `json:"type"`
}

type parameterImpact struct {
	Callable   graph.SymbolRef    `json:"callable"`
	Before     callableSignature  `json:"before"`
	After      callableSignature  `json:"after"`
	CallSites  []callSiteImpact   `json:"callSites"`
	Contracts  []contractImpact   `json:"contracts"`
	Structural []structuralImpact `json:"structural"`
}

type callableSignature struct {
	Parameters []parameter `json:"parameters"`
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

func handleParameterImpact(w http.ResponseWriter, r *http.Request, analysis *goanalyzer.Analysis) {
	if analysis == nil {
		writeError(w, http.StatusInternalServerError, "Go analysis is unavailable")
		return
	}
	var request parameterImpactRequest
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
	impact, err := analysis.AnalyzeParameterChange(request.Callable, proposed)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, presentParameterImpact(impact))
}

func presentParameterImpact(source query.ParameterChangeImpact) parameterImpact {
	result := parameterImpact{
		Callable:   source.Callable,
		Before:     presentCallableSignature(source.Before),
		After:      presentCallableSignature(source.After),
		CallSites:  make([]callSiteImpact, 0, len(source.CallSites)),
		Contracts:  make([]contractImpact, 0, len(source.Contracts)),
		Structural: make([]structuralImpact, 0, len(source.Structural)),
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
	result := callableSignature{Parameters: make([]parameter, 0, len(source.Parameters)), Variadic: source.Variadic}
	for _, sourceParameter := range source.Parameters {
		result.Parameters = append(result.Parameters, parameter{Name: sourceParameter.Name, Type: presentGoTypeRef(sourceParameter.Type)})
	}
	return result
}

func presentGoTypeRef(source query.GoTypeRef) goTypeRef {
	return goTypeRef{Display: source.Display, Symbol: source.Symbol}
}
