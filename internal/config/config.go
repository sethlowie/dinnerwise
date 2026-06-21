// Package config loads runtime configuration from the environment. main loads
// .env (via godotenv) before calling Load, so values may originate there.
package config

import "os"

// Config holds runtime settings. Slice 6b extends this with OTLP/Sigil fields.
type Config struct {
	OpenAIAPIKey string
	OpenAIModel  string
}

// Load reads configuration from the process environment.
func Load() Config {
	model := os.Getenv("OPENAI_MODEL")
	if model == "" {
		model = "gpt-5-nano"
	}
	return Config{
		OpenAIAPIKey: os.Getenv("OPENAI_API_KEY"),
		OpenAIModel:  model,
	}
}

// HasOpenAI reports whether a real OpenAI agent can be constructed.
func (c Config) HasOpenAI() bool { return c.OpenAIAPIKey != "" }
