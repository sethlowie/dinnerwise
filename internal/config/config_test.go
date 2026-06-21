package config

import "testing"

func TestLoadDefaultsModel(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("OPENAI_MODEL", "")
	c := Load()
	if c.OpenAIModel != "gpt-5-nano" {
		t.Fatalf("default model = %q, want gpt-5-nano", c.OpenAIModel)
	}
	if c.HasOpenAI() {
		t.Fatal("HasOpenAI() should be false with empty key")
	}
}

func TestLoadReadsEnv(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "sk-test")
	t.Setenv("OPENAI_MODEL", "gpt-5")
	c := Load()
	if !c.HasOpenAI() {
		t.Fatal("HasOpenAI() should be true")
	}
	if c.OpenAIModel != "gpt-5" {
		t.Fatalf("model = %q, want gpt-5", c.OpenAIModel)
	}
}

func TestObservability(t *testing.T) {
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "")
	t.Setenv("OTEL_SERVICE_NAME", "")
	c := Load()
	if c.HasObservability() {
		t.Fatal("HasObservability should be false with no endpoint")
	}
	if c.ServiceName != "dinnerwise" {
		t.Fatalf("default ServiceName = %q, want dinnerwise", c.ServiceName)
	}

	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "http://localhost:4318")
	t.Setenv("OTEL_SERVICE_NAME", "custom")
	c = Load()
	if !c.HasObservability() || c.OTLPEndpoint != "http://localhost:4318" {
		t.Fatalf("HasObservability/endpoint wrong: %+v", c)
	}
	if c.ServiceName != "custom" {
		t.Fatalf("ServiceName = %q, want custom", c.ServiceName)
	}
}
