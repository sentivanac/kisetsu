package main

import (
	"testing"

	"github.com/wailsapp/wails/v2/pkg/options/linux"
)

func TestParseGPUPolicy(t *testing.T) {
	tests := map[string]linux.WebviewGpuPolicy{
		"":         linux.WebviewGpuPolicyOnDemand,
		"ondemand": linux.WebviewGpuPolicyOnDemand,
		"Always":   linux.WebviewGpuPolicyAlways,
		" never ":  linux.WebviewGpuPolicyNever,
	}
	for in, want := range tests {
		got, err := parseGPUPolicy(in)
		if err != nil {
			t.Errorf("parseGPUPolicy(%q) error: %v", in, err)
		}
		if got != want {
			t.Errorf("parseGPUPolicy(%q) = %v, want %v", in, got, want)
		}
	}

	if got, err := parseGPUPolicy("turbo"); err == nil || got != linux.WebviewGpuPolicyOnDemand {
		t.Errorf("parseGPUPolicy(\"turbo\") = %v, %v; want ondemand and an error", got, err)
	}
}
