/*
Package core provides shared functionality for Eino chains.
*/
package core

import (
	"bytes"
	"context"
	"fmt"
	"text/template"
	"time"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
)

// DeterministicChain is a reusable pipeline: Map -> Template -> Model -> Parser -> Output
type DeterministicChain[T any] struct {
	chain compose.Runnable[map[string]any, T]
	name  string
}

// NewDeterministicChain creates a standardized Eino chain for deterministic tasks.
func NewDeterministicChain[T any](
	ctx context.Context,
	name string,
	chatModel model.BaseChatModel,
	templateStr string,
) (*DeterministicChain[T], error) {

	// 1. Template Node (Custom Lambda)
	tmpl, err := template.New(name).Parse(templateStr)
	if err != nil {
		return nil, fmt.Errorf("parse template: %w", err)
	}

	templateFunc := func(ctx context.Context, input map[string]any) ([]*schema.Message, error) {
		var buf bytes.Buffer
		if err := tmpl.Execute(&buf, input); err != nil {
			return nil, fmt.Errorf("execute template: %w", err)
		}
		return []*schema.Message{
			{Role: schema.User, Content: buf.String()},
		}, nil
	}

	// 2. Model Node (Lambda Adapter)
	// We wrap BaseChatModel in a lambda to accept models that don't support tools (BindTools)
	modelFunc := func(ctx context.Context, input []*schema.Message) (*schema.Message, error) {
		return chatModel.Generate(ctx, input)
	}

	// 3. Parser function (wrapping our generic ParseJSONResponse)
	parserFunc := func(ctx context.Context, output *schema.Message) (T, error) {
		return ParseJSONResponse[T](output.Content)
	}

	// 4. Chain Construction using Graph
	graph := compose.NewGraph[map[string]any, T]()

	_ = graph.AddLambdaNode("prompt", compose.InvokableLambda(templateFunc))
	_ = graph.AddLambdaNode("model", compose.InvokableLambda(modelFunc))
	_ = graph.AddLambdaNode("parser", compose.InvokableLambda(parserFunc))

	_ = graph.AddEdge(compose.START, "prompt")
	_ = graph.AddEdge("prompt", "model")
	_ = graph.AddEdge("model", "parser")
	_ = graph.AddEdge("parser", compose.END)

	compiledChain, err := graph.Compile(ctx)
	if err != nil {
		return nil, fmt.Errorf("compile chain: %w", err)
	}

	return &DeterministicChain[T]{
		chain: compiledChain,
		name:  name,
	}, nil
}

// Invoke executes the chain with manual timing.
func (c *DeterministicChain[T]) Invoke(ctx context.Context, input map[string]any) (T, string, time.Duration, error) {
	start := time.Now()

	// In Eino v0.7.x, callbacks are often handled via Options or Context injection differently.
	// For simplicity in this fix, we will just run the chain.
	// If needed, we can add `callbacks.WithHandler` if the API supports it in Invoke,
	// or rely on Eino's default context propagation if headers are set.

	// Execute chain
	// Run(ctx, input, opts...) is the standard for Runnable
	output, err := c.chain.Invoke(ctx, input)
	duration := time.Since(start)

	return output, "", duration, err
}
