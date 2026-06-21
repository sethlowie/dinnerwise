// Package config loads runtime configuration from the environment. main loads
// .env (via godotenv) before calling Load, so values may originate there.
package config

import "os"

// Config holds runtime settings.
type Config struct {
	OpenAIAPIKey string
	OpenAIModel  string
	OTLPEndpoint string
	ServiceName  string
}

// Load reads configuration from the process environment.
func Load() Config {
	model := os.Getenv("OPENAI_MODEL")
	if model == "" {
		model = "gpt-5-nano"
	}
	service := os.Getenv("OTEL_SERVICE_NAME")
	if service == "" {
		service = "dinnerwise"
	}
	return Config{
		OpenAIAPIKey: os.Getenv("OPENAI_API_KEY"),
		OpenAIModel:  model,
		OTLPEndpoint: os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT"),
		ServiceName:  service,
	}
}

// HasOpenAI reports whether a real OpenAI agent can be constructed.
func (c Config) HasOpenAI() bool { return c.OpenAIAPIKey != "" }

// HasObservability reports whether OTel export is configured.
func (c Config) HasObservability() bool { return c.OTLPEndpoint != "" }
