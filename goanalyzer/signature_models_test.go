package goanalyzer

import "testing"

func TestMethodExposureString(t *testing.T) {
	tests := []struct {
		exposure MethodExposure
		want     string
	}{
		{exposure: MethodExposureUnknown, want: "unknown"},
		{exposure: MethodExposureValue, want: "value"},
		{exposure: MethodExposurePointerOnly, want: "pointer only"},
		{exposure: MethodExposure(255), want: "unknown"},
	}
	for _, test := range tests {
		if got := test.exposure.String(); got != test.want {
			t.Errorf("MethodExposure(%d).String() = %q, want %q", test.exposure, got, test.want)
		}
	}
}
