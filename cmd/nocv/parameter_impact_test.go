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
