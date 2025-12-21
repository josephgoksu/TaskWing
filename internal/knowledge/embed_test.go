package knowledge

import (
	"context"
	"errors"
	"testing"

	"github.com/cloudwego/eino/components/embedding"
	"github.com/josephgoksu/TaskWing/internal/llm"
)

// MockEmbedder implements embedding.Embedder for testing
type MockEmbedder struct {
	Vectors [][]float64
	Err     error
}

func (m *MockEmbedder) EmbedStrings(ctx context.Context, texts []string, opts ...embedding.Option) ([][]float64, error) {
	if m.Err != nil {
		return nil, m.Err
	}
	return m.Vectors, nil
}

func TestGenerateEmbedding(t *testing.T) {
	originalFactory := embeddingModelFactory
	defer func() { embeddingModelFactory = originalFactory }()

	tests := []struct {
		name    string
		mock    *MockEmbedder
		wantErr bool
		wantLen int
	}{
		{
			name: "successful embedding",
			mock: &MockEmbedder{
				Vectors: [][]float64{{0.1, 0.2, 0.3}},
			},
			wantErr: false,
			wantLen: 3,
		},
		{
			name: "embedding error",
			mock: &MockEmbedder{
				Err: errors.New("provider error"),
			},
			wantErr: true,
			wantLen: 0,
		},
		{
			name: "empty response",
			mock: &MockEmbedder{
				Vectors: [][]float64{},
			},
			wantErr: true,
			wantLen: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			embeddingModelFactory = func(ctx context.Context, cfg llm.Config) (embedding.Embedder, error) {
				return tt.mock, nil
			}

			got, err := GenerateEmbedding(context.Background(), "test text", llm.Config{})
			if (err != nil) != tt.wantErr {
				t.Errorf("GenerateEmbedding() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if len(got) != tt.wantLen {
					t.Errorf("GenerateEmbedding() got length %d, want %d", len(got), tt.wantLen)
				}
				// Verify float conversion using epsilon
				diff := float64(got[0]) - tt.mock.Vectors[0][0]
				if diff < 0 {
					diff = -diff
				}
				if diff > 0.00001 {
					t.Errorf("GenerateEmbedding() value mismatch: got %f, want %f", got[0], tt.mock.Vectors[0][0])
				}
			}
		})
	}
}

func TestCosineSimilarity(t *testing.T) {
	tests := []struct {
		name string
		a    []float32
		b    []float32
		want float32
	}{
		{
			name: "identical vectors",
			a:    []float32{1, 0, 0},
			b:    []float32{1, 0, 0},
			want: 1.0,
		},
		{
			name: "orthogonal vectors",
			a:    []float32{1, 0, 0},
			b:    []float32{0, 1, 0},
			want: 0.0,
		},
		{
			name: "opposite vectors",
			a:    []float32{1, 0, 0},
			b:    []float32{-1, 0, 0},
			want: -1.0,
		},
		{
			name: "different lengths",
			a:    []float32{1, 0},
			b:    []float32{1, 0, 0},
			want: 0.0,
		},
		{
			name: "empty vectors",
			a:    []float32{},
			b:    []float32{},
			want: 0.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CosineSimilarity(tt.a, tt.b)
			// Float comparison with epsilon
			diff := got - tt.want
			if diff < 0 {
				diff = -diff
			}
			if diff > 0.0001 {
				t.Errorf("CosineSimilarity() = %v, want %v", got, tt.want)
			}
		})
	}
}
