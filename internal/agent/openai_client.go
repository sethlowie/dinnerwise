package agent

import (
	"context"

	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
	"github.com/openai/openai-go/v3/responses"
	"github.com/openai/openai-go/v3/shared"
)

type openAIClient struct {
	client openai.Client
	model  string
}

func newOpenAIClient(apiKey, model string) llmClient {
	c := openai.NewClient(option.WithAPIKey(apiKey))
	return &openAIClient{client: c, model: model}
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
		}
	}
	params.Input = responses.ResponseNewParamsInputUnion{OfInputItemList: input}

	resp, err := o.client.Responses.New(ctx, params)
	if err != nil {
		return llmTurn{}, err
	}

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
