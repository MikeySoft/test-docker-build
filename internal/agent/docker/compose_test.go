package docker

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestInjectFlotillaLabelsAddsLabels(t *testing.T) {
	input := `
services:
  app:
    image: nginx:latest
`
	result, err := injectFlotillaLabels(input, "test-stack")
	if err != nil {
		t.Fatalf("injectFlotillaLabels returned error: %v", err)
	}

	var parsed map[string]any
	if err := yaml.Unmarshal([]byte(result), &parsed); err != nil {
		t.Fatalf("failed to parse output yaml: %v", err)
	}

	services := parsed["services"].(map[string]any)
	app := services["app"].(map[string]any)
	labels := app["labels"].(map[string]any)
	if labels[flotillaManagedLabel] != "true" {
		t.Fatalf("expected %s label set to true", flotillaManagedLabel)
	}
	if labels[flotillaStackNameLabel] != "test-stack" {
		t.Fatalf("expected stack name label to equal test-stack, got %v", labels[flotillaStackNameLabel])
	}
	if _, ok := labels[flotillaDeployedLabel]; !ok {
		t.Fatalf("expected deployment timestamp label to be present")
	}
}

func TestInjectFlotillaLabelsConvertsArrayLabels(t *testing.T) {
	input := `
services:
  worker:
    image: alpine
    labels:
      - "custom.label=value"
`
	result, err := injectFlotillaLabels(input, "array-stack")
	if err != nil {
		t.Fatalf("injectFlotillaLabels returned error: %v", err)
	}

	var parsed map[string]any
	if err := yaml.Unmarshal([]byte(result), &parsed); err != nil {
		t.Fatalf("failed to parse output yaml: %v", err)
	}
	labels := parsed["services"].(map[string]any)["worker"].(map[string]any)["labels"].(map[string]any)
	if labels["custom.label"] != "value" {
		t.Fatalf("expected custom label preserved, got %v", labels["custom.label"])
	}
	if labels[flotillaManagedLabel] != "true" {
		t.Fatalf("expected flotilla managed label to be set")
	}
}

func TestInjectFlotillaLabelsNoServices(t *testing.T) {
	input := `
version: "3.9"
`
	result, err := injectFlotillaLabels(input, "no-services")
	if err != nil {
		t.Fatalf("injectFlotillaLabels returned error: %v", err)
	}
	if strings.TrimSpace(result) != strings.TrimSpace(input) {
		t.Fatalf("expected compose content unchanged when no services section present")
	}
}
