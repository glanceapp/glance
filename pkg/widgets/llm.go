package widgets

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
	Highlight string `json:"highlight" description:"A short and concise summary for why this item is a good match for the query. No any unecessary filler text (e.g. 'This includes...'). Must be two or three short sentences max."`
}

type completionResponse struct {
	Matches []feedMatch `json:"matches"`
}

// filterFeed returns the IDs of feed entries that match the query
func (llm *LLM) filterFeed(ctx context.Context, feed []feedEntry, query string) ([]feedMatch, error) {
	prompt := strings.Builder{}

	prompt.WriteString(`
## Role
You are an activity feed personalization assistant, 
that helps the user find and focus on the most relevant content.

You are given a list of feed entries with id, title, and description fields - given the natural language query,
you should rank these entries based on how well they match the query on a scale of 0 to 10.

## Relevance scoring
For each entry, use the associated highlight text as a reflective summary to help assess how well the entry matches the user’s query. Follow these rules:
	•	If the highlight does not clearly explain how the entry is relevant to the user’s query, assign a low relevance score (≤ 3/10), even if the entry is interesting on its own.
	•	If the highlight is vague or generic (e.g., asks a broad question or restates the title), treat it as insufficient evidence of relevance unless the original content clearly supports the query.
	•	Strong highlights should:
	•	Explicitly mention key topics, entities, or themes from the user query.
	•	Clearly describe how the entry contributes useful, novel, or actionable insight toward the user’s intent.
	•	Use the highlight as a justification tool: If it doesn’t support the match, downgrade the score. If it adds clarity and alignment, consider upgrading the score.

Always base the relevance score on how well the highlight connects the entry to the user’s information needs, not just on the entry’s popularity or standalone quality.
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
