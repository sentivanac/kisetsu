package main

import (
	"fmt"
	"strings"

	"github.com/wailsapp/wails/v2/pkg/options/linux"
)

// gpuPolicyEnv overrides WebView hardware acceleration. Setting it to
// "never" is the workaround for systems where the window stays blank
// (https://github.com/wailsapp/wails/issues/2977).
const gpuPolicyEnv = "KISETSU_WEBVIEW_GPU"

// parseGPUPolicy converts ondemand, always or never (case-insensitive)
// into a WebView GPU policy. An empty value means ondemand, WebKit's own
// default, which keeps scrolling through image grids smooth.
func parseGPUPolicy(value string) (linux.WebviewGpuPolicy, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "ondemand":
		return linux.WebviewGpuPolicyOnDemand, nil
	case "always":
		return linux.WebviewGpuPolicyAlways, nil
	case "never":
		return linux.WebviewGpuPolicyNever, nil
	}
	return linux.WebviewGpuPolicyOnDemand, fmt.Errorf("unknown %s value %q (use ondemand, always or never)", gpuPolicyEnv, value)
}
