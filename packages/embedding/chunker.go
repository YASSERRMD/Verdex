package embedding

import "strings"

// Chunker splits long text into overlapping segments suitable for embedding.
// The default tokenizer is a naive whitespace splitter; swap in a proper
// tokenizer by embedding Chunker in a wrapper that overrides Tokenize.
type Chunker struct {
	// Tokenize converts text into tokens.  When nil, strings.Fields is used.
	Tokenize func(text string) []string

	// Detokenize converts tokens back into a string.  When nil, a space-join
	// is used.
	Detokenize func(tokens []string) string
}

func (c *Chunker) tokenize(text string) []string {
	if c.Tokenize != nil {
		return c.Tokenize(text)
	}
	return strings.Fields(text)
}

func (c *Chunker) detokenize(tokens []string) string {
	if c.Detokenize != nil {
		return c.Detokenize(tokens)
	}
	return strings.Join(tokens, " ")
}

// Split divides text into a slice of chunk strings according to cfg.
//
// Strategy:
//  1. If cfg.SplitOn is non-empty, the text is first partitioned on that
//     boundary sequence (e.g. "\n\n" for paragraphs, ". " for sentences).
//     Adjacent segments are merged until the next merge would exceed
//     MaxTokens, at which point a new chunk is started.
//  2. Each chunk (however derived) is trimmed to MaxTokens tokens via a hard
//     split if still over the limit.
//  3. cfg.Overlap trailing tokens from each chunk are prepended to the next.
func (c *Chunker) Split(text string, cfg ChunkConfig) []string {
	if cfg.MaxTokens <= 0 {
		cfg.MaxTokens = 512
	}
	if cfg.Overlap < 0 {
		cfg.Overlap = 0
	}
	if cfg.Overlap >= cfg.MaxTokens {
		cfg.Overlap = cfg.MaxTokens / 2
	}

	// Step 1: split on preferred boundary.
	var segments [][]string // each segment is a token slice
	if cfg.SplitOn != "" {
		parts := strings.Split(text, cfg.SplitOn)
		for _, p := range parts {
			p = strings.TrimSpace(p)
			if p == "" {
				continue
			}
			segments = append(segments, c.tokenize(p))
		}
	} else {
		segments = [][]string{c.tokenize(text)}
	}

	if len(segments) == 0 {
		return nil
	}

	// Step 2: merge short segments and hard-split long ones into chunks.
	var chunks []string
	var carry []string // overlap tokens from previous chunk

	buf := make([]string, 0, cfg.MaxTokens)
	buf = append(buf, carry...)

	for _, seg := range segments {
		// If adding this segment exceeds MaxTokens, flush current buf first.
		if len(buf)+len(seg) > cfg.MaxTokens && len(buf) > 0 {
			chunks = append(chunks, c.detokenize(buf))
			// Compute overlap: last cfg.Overlap tokens of buf.
			overlap := overlapTokens(buf, cfg.Overlap)
			buf = make([]string, 0, cfg.MaxTokens)
			buf = append(buf, overlap...)
		}

		// If the segment alone exceeds MaxTokens, hard-split it.
		for len(seg) > 0 {
			remaining := cfg.MaxTokens - len(buf)
			if remaining <= 0 {
				chunks = append(chunks, c.detokenize(buf))
				overlap := overlapTokens(buf, cfg.Overlap)
				buf = make([]string, 0, cfg.MaxTokens)
				buf = append(buf, overlap...)
				remaining = cfg.MaxTokens - len(buf)
			}
			take := remaining
			if take > len(seg) {
				take = len(seg)
			}
			buf = append(buf, seg[:take]...)
			seg = seg[take:]
		}
	}

	// Flush remaining tokens.
	if len(buf) > 0 {
		chunks = append(chunks, c.detokenize(buf))
	}

	return chunks
}

// overlapTokens returns the last n tokens of toks, or all of toks if len < n.
func overlapTokens(toks []string, n int) []string {
	if n <= 0 || len(toks) == 0 {
		return nil
	}
	if n >= len(toks) {
		out := make([]string, len(toks))
		copy(out, toks)
		return out
	}
	out := make([]string, n)
	copy(out, toks[len(toks)-n:])
	return out
}
