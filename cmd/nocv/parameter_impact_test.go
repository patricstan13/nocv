package main

import (
	"bytes"
	"context"
	"path/filepath"
	"strings"
	"testing"

	"nocv/goanalyzer"
)

func TestParseImpactParamsInvocation(t *testing.T) {
	invocation, err := parseInvocation([]string{
		"impact-params", "./...", "example.com/app::Use",
		"--param", "string", "--param", "*log.Logger", "--variadic",
	})
	if err != nil {
		t.Fatal(err)
	}
	if invocation.name != "impact-params" || len(invocation.values) != 6 {
		t.Fatalf("invocation = %+v", invocation)
	}

	if _, err := parseInvocation([]string{"impact-params", "./..."}); err == nil ||
		!strings.Contains(err.Error(), "usage: nocv impact-params") {
		t.Fatalf("missing callable error = %v", err)
	}
}

func TestImpactParamsCommandRendersConcreteCallSites(t *testing.T) {
	dir, err := filepath.Abs("../../goanalyzer/testdata/parameterimpact")
	if err != nil {
		t.Fatal(err)
	}
	analysis, err := goanalyzer.LoadAnalysis(context.Background(), dir, "./...")
	if err != nil {
		t.Fatal(err)
	}
	invocation := invocation{
		name:    "impact-params",
		pattern: dir + "/...",
		values:  []string{"example.com/parameterimpact::UseID", "--param", "string"},
	}

	var output bytes.Buffer
	if err := executeCommandWithAnalysis(&output, analysis.Graph(), analysis, invocation); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"SIGNATURE CHANGE",
		"UseID(example.com/parameterimpact.ID)",
		"UseID(string)",
		"CALL-SITE IMPACT",
		"CONTRACT IMPACT",
		"STRUCTURAL IMPACT",
		"(none)",
		"INCOMPATIBLE",
		"COMPATIBLE",
		"example.com/parameterimpact::CallsID",
		"argument 1: example.com/parameterimpact.ID -> string",
		"impact.go @ ",
	} {
		if !strings.Contains(output.String(), want) {
			t.Errorf("output lacks %q:\n%s", want, output.String())
		}
	}
}

func TestImpactParamsCommandRendersPromotedMethodStructuralImpact(t *testing.T) {
	dir, err := filepath.Abs("../../goanalyzer/testdata/parameterimpact")
	if err != nil {
		t.Fatal(err)
	}
	analysis, err := goanalyzer.LoadAnalysis(context.Background(), dir, "./...")
	if err != nil {
		t.Fatal(err)
	}
	invocation := invocation{
		name:    "impact-params",
		pattern: dir + "/...",
		values:  []string{"example.com/parameterimpact::PromotionBase::Change", "--param", "string"},
	}

	var output bytes.Buffer
	if err := executeCommandWithAnalysis(&output, analysis.Graph(), analysis, invocation); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"STRUCTURAL IMPACT",
		"PROMOTED METHOD CHANGED",
		"PromotionOuter exposes Change from PromotionBase.Change",
		"PromotionWrapper exposes Change from PromotionBase.Change",
	} {
		if !strings.Contains(output.String(), want) {
			t.Errorf("output lacks %q:\n%s", want, output.String())
		}
	}
	for _, unwanted := range []string{"PromotionShadow exposes", "PromotionAmbiguous exposes"} {
		if strings.Contains(output.String(), unwanted) {
			t.Errorf("output unexpectedly contains %q:\n%s", unwanted, output.String())
		}
	}

	invocation.values = []string{"example.com/parameterimpact::PointerBase::Touch", "--param", "string"}
	output.Reset()
	if err := executeCommandWithAnalysis(&output, analysis.Graph(), analysis, invocation); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"PointerEmbed exposes Touch from PointerBase.Touch",
		"*ValueEmbed exposes Touch from PointerBase.Touch",
		"*RecursivePointerMid exposes Touch from PointerBase.Touch",
		"*RecursivePointerOuter exposes Touch from PointerBase.Touch",
	} {
		if !strings.Contains(output.String(), want) {
			t.Errorf("pointer output lacks %q:\n%s", want, output.String())
		}
	}
}

func TestImpactParamsCommandRendersConcreteAndInterfaceContractImpact(t *testing.T) {
	dir, err := filepath.Abs("../../goanalyzer/testdata/parameterimpact")
	if err != nil {
		t.Fatal(err)
	}
	analysis, err := goanalyzer.LoadAnalysis(context.Background(), dir, "./...")
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name     string
		callable string
		want     []string
	}{
		{
			name:     "concrete method",
			callable: "example.com/parameterimpact::Service::Save",
			want: []string{
				"Service no longer implements ExtendedStore",
				"Service no longer implements Saver",
				"Service no longer implements Store",
				"Service.Save",
				"Store.Save",
			},
		},
		{
			name:     "interface method",
			callable: "example.com/parameterimpact::Store::Save",
			want: []string{
				"PointerStore no longer implements Store",
				"Service no longer implements ExtendedStore",
				"Service no longer implements Store",
				"PointerStore.Save",
				"Store.Save",
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			invocation := invocation{
				name:    "impact-params",
				pattern: dir + "/...",
				values:  []string{test.callable, "--param", "string"},
			}
			var output bytes.Buffer
			if err := executeCommandWithAnalysis(&output, analysis.Graph(), analysis, invocation); err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(output.String(), "CONTRACT IMPACT\n\nLOST IMPLEMENTATION") {
				t.Fatalf("output lacks contract section:\n%s", output.String())
			}
			for _, want := range test.want {
				if !strings.Contains(output.String(), want) {
					t.Errorf("output lacks %q:\n%s", want, output.String())
				}
			}
		})
	}
}

func TestParseProposedSignatureRejectsMalformedFlags(t *testing.T) {
	tests := []struct {
		values []string
		want   string
	}{
		{values: []string{"callable", "--param"}, want: "--param requires"},
		{values: []string{"callable", "--variadic", "--variadic"}, want: "only once"},
		{values: []string{"callable", "--unknown"}, want: "unknown impact-params option"},
	}
	for _, test := range tests {
		_, _, err := parseProposedSignature(test.values)
		if err == nil || !strings.Contains(err.Error(), test.want) {
			t.Fatalf("parseProposedSignature(%v) error = %v, want containing %q", test.values, err, test.want)
		}
	}
}
