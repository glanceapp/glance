package glance

import (
	"context"
	"fmt"
	"strings"

	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/llms/openai"
	"github.com/tmc/langchaingo/outputparser"
)

type LLM struct {
	model llms.Model
}

func NewLLM() (*LLM, error) {
	model, err := openai.New(
		openai.WithModel("gpt-4o-mini"),
	)
	if err != nil {
		return nil, err
	}
	return &LLM{model: model}, nil
}

type feedMatch struct {
	ID        string `json:"id"`
	Score     int    `json:"score" description:"How closely this item matches the query, from 0 to 10."`
	Highlight string `json:"highlight" description:"A short explanation for why this item is a good match for the query. Short and concise, without any unecessary filler text (e.g. 'This includes...')."`
}

type completionResponse struct {
	Matches []feedMatch `json:"matches"`
}

// filterFeed returns the IDs of feed entries that match the query
func (llm *LLM) filterFeed(ctx context.Context, feed []feedEntry, query string) ([]feedMatch, error) {
	prompt := strings.Builder{}

	prompt.WriteString(`
You are an activity feed personalization assistant, 
that helps the user find and focus on the most relevant content.

You are given a list of feed entries with id, title, and description fields - given the natural language query,
you should rank these entries based on how well they match the query on a scale of 0 to 10.
`)
	prompt.WriteString(fmt.Sprintf("filter query: %s\n", query))

	for _, entry := range feed {
		prompt.WriteString(fmt.Sprintf("id: %s\n", entry.ID))
		prompt.WriteString(fmt.Sprintf("title: %s\n", entry.Title))
		prompt.WriteString(fmt.Sprintf("description: %s\n", entry.Description))
		prompt.WriteString("\n")
	}

	parser, err := outputparser.NewDefined(completionResponse{})
	if err != nil {
		return nil, fmt.Errorf("creating parser: %w", err)
	}

	prompt.WriteString(fmt.Sprintf("\n\n%s", parser.GetFormatInstructions()))

	out, err := llms.GenerateFromSinglePrompt(
		ctx,
		llm.model,
		prompt.String(),
	)
	if err != nil {
		return nil, fmt.Errorf("generating completion: %w", err)
	}

	response, err := parser.Parse(out)
	if err != nil {
		return nil, fmt.Errorf("parsing response: %w", err)
	}

	return response.Matches, nil
}
