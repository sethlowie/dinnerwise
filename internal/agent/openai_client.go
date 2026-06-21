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

func (o *openAIClient) Respond(ctx context.Context, prev string, toolOutputs []llmToolOutput, userText string) (llmTurn, error) {
	params := responses.ResponseNewParams{
		Model: shared.ResponsesModel(o.model),
		Tools: toolDefs(),
		// Reasoning effort low keeps fast models snappy.
		Reasoning: shared.ReasoningParam{Effort: shared.ReasoningEffortLow},
	}

	if prev != "" {
		params.PreviousResponseID = openai.String(prev)
	}

	var items responses.ResponseInputParam
	if userText != "" {
		// First turn: set system instructions and add the user message.
		params.Instructions = openai.String(systemPrompt)
		items = append(items, responses.ResponseInputItemParamOfMessage(userText, responses.EasyInputMessageRoleUser))
	}
	for _, out := range toolOutputs {
		items = append(items, responses.ResponseInputItemParamOfFunctionCallOutput(out.CallID, out.Output))
	}
	if len(items) > 0 {
		params.Input = responses.ResponseNewParamsInputUnion{OfInputItemList: items}
	}

	resp, err := o.client.Responses.New(ctx, params)
	if err != nil {
		return llmTurn{}, err
	}

	turn := llmTurn{ResponseID: resp.ID}
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
