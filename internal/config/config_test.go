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
