package agent

import (
	"context"

	"github.com/grafana/sigil-sdk/go/sigil"
	sigilopenai "github.com/grafana/sigil-sdk/go-providers/openai"
	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
	"github.com/openai/openai-go/v3/responses"
	"github.com/openai/openai-go/v3/shared"
)

type openAIClient struct {
	client openai.Client
	model  string
	sigil  *sigil.Client
}

func newOpenAIClient(apiKey, model string, sclient *sigil.Client) llmClient {
	c := openai.NewClient(option.WithAPIKey(apiKey))
	return &openAIClient{client: c, model: model, sigil: sclient}
}

func (o *openAIClient) Respond(ctx context.Context, items []llmItem) (llmTurn, error) {
	// Stateless: the org enforces Zero Data Retention, so we cannot chain with
	// previous_response_id. Resend the full conversation each call, with
	// store=false and the system instructions every time.
	params := responses.ResponseNewParams{
		Model:        shared.ResponsesModel(o.model),
		Tools:        toolDefs(),
		Instructions: openai.String(systemPrompt),
		Store:        openai.Bool(false),
		// Reasoning effort low keeps fast models snappy.
		Reasoning: shared.ReasoningParam{Effort: shared.ReasoningEffortLow},
	}

	var input responses.ResponseInputParam
	for _, it := range items {
		switch {
		case it.ToolCall != nil:
			input = append(input, responses.ResponseInputItemParamOfFunctionCall(
				it.ToolCall.Arguments, it.ToolCall.CallID, it.ToolCall.Name))
		case it.ToolOutput != nil:
			input = append(input, responses.ResponseInputItemParamOfFunctionCallOutput(
				it.ToolOutput.CallID, it.ToolOutput.Output))
		case it.UserText != "":
			input = append(input, responses.ResponseInputItemParamOfMessage(
				it.UserText, responses.EasyInputMessageRoleUser))
		case it.AssistantText != "":
			input = append(input, responses.ResponseInputItemParamOfMessage(
				it.AssistantText, responses.EasyInputMessageRoleAssistant))
		}
	}
	params.Input = responses.ResponseNewParamsInputUnion{OfInputItemList: input}

	var resp *responses.Response
	var err error
	if o.sigil != nil {
		resp, err = sigilopenai.ResponsesNew(ctx, o.sigil, o.client, params)
	} else {
		resp, err = o.client.Responses.New(ctx, params)
	}
	if err != nil {
		return llmTurn{}, err
	}
	// Approximate $ cost from usage (tokens themselves are recorded by Sigil).
	recordCost(ctx, o.model, resp.Usage.InputTokens, resp.Usage.OutputTokens)

	turn := llmTurn{}
	for _, item := range resp.Output {
		if item.Type == "function_call" {
			fc := item.AsFunctionCall()
			turn.ToolCalls = append(turn.ToolCalls, llmToolCall{
				CallID:    fc.CallID,
				Name:      fc.Name,
				Arguments: fc.Arguments,
			})
		}
	}
	turn.Text = resp.OutputText()
	return turn, nil
}
