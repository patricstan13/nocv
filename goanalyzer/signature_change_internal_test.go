package goanalyzer

import "testing"

func TestCompareCompilerDiagnosticsDirtyBaseline(t *testing.T) {
	baseline := []compilerDiagnostic{
		{Package: "example.com/app", Pos: "/tmp/app.go:10:2", Message: "existing problem"},
	}
	changed := []compilerDiagnostic{
		{Package: "example.com/app", Pos: "/tmp/app.go:12:2", Message: "existing   problem"},
		{Package: "example.com/app", Pos: "/tmp/app.go:20:2", Message: "new problem"},
	}
	delta := compareCompilerDiagnostics(baseline, changed)
	if len(delta) != 2 || delta[0].classification != DiagnosticUncertain || delta[1].classification != DiagnosticNew {
		t.Fatalf("dirty delta = %#v", delta)
	}
}

func TestCompareCompilerDiagnosticsSuppressesExactBaselineOccurrence(t *testing.T) {
	diagnostic := compilerDiagnostic{Package: "example.com/app", Pos: "/tmp/app.go:10:2", Message: "existing problem"}
	if delta := compareCompilerDiagnostics([]compilerDiagnostic{diagnostic}, []compilerDiagnostic{diagnostic}); len(delta) != 0 {
		t.Fatalf("exact baseline diagnostic returned as consequence: %#v", delta)
	}
}
