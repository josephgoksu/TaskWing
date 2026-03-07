// Package compress provides output compression for CLI command results.
// It reduces token usage by filtering, deduplicating, and truncating output.
package compress

// Filter transforms raw output bytes into compressed output.
type Filter func([]byte) []byte

// Pipeline runs a sequence of filters on input data.
type Pipeline struct {
	filters []Filter
}

// NewPipeline creates a pipeline from the given filters.
func NewPipeline(filters ...Filter) *Pipeline {
	return &Pipeline{filters: filters}
}

// Run applies all filters in order to the input.
func (p *Pipeline) Run(input []byte) []byte {
	data := input
	for _, f := range p.filters {
		data = f(data)
		if len(data) == 0 {
			return data
		}
	}
	return data
}

// Stats tracks compression metrics.
type Stats struct {
	InputBytes  int
	OutputBytes int
	Command     string
}

// Ratio returns the compression ratio (0.0 = perfect, 1.0 = no compression).
func (s Stats) Ratio() float64 {
	if s.InputBytes == 0 {
		return 1.0
	}
	return float64(s.OutputBytes) / float64(s.InputBytes)
}

// Saved returns the percentage of bytes saved.
func (s Stats) Saved() float64 {
	return (1.0 - s.Ratio()) * 100
}

// Compress runs the appropriate pipeline for a command and returns compressed output + stats.
func Compress(command string, raw []byte) ([]byte, Stats) {
	pipeline := ForCommand(command)
	output := pipeline.Run(raw)
	return output, Stats{
		InputBytes:  len(raw),
		OutputBytes: len(output),
		Command:     command,
	}
}
