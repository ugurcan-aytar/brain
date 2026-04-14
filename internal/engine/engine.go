// Package engine wraps recall.Engine + its embedder into a single
// lifecycle brain commands can open at the top of a handler, defer-close,
// and hand to the retriever / command layer. The embedder is lazy so
// BM25-only paths don't pay for GGUF load / API construction.
package engine

import (
	"errors"
	"fmt"
	"sync"

	"github.com/ugurcan-aytar/recall/pkg/recall"
)

// Engine bundles a *recall.Engine with a lazily-resolved Embedder. Safe
// for concurrent use once opened.
type Engine struct {
	rcl *recall.Engine

	embOnce sync.Once
	emb     recall.Embedder
	embErr  error
}

// Open creates a recall engine at the default DB path (or
// $RECALL_DB_PATH when set). Additional recall.Option values can be
// passed through — tests use recall.WithDBPath to sandbox a temp file.
func Open(opts ...recall.Option) (*Engine, error) {
	r, err := recall.NewEngine(opts...)
	if err != nil {
		return nil, fmt.Errorf("open recall engine: %w", err)
	}
	return &Engine{rcl: r}, nil
}

// Recall exposes the underlying recall.Engine so callers can invoke the
// full public API (AddCollection, Index, SearchHybrid, Get, …).
func (e *Engine) Recall() *recall.Engine { return e.rcl }

// Embedder returns the active embedder, constructed on first call.
//
// Returns (nil, nil) when no embedder can be constructed AND the failure
// is a graceful-degradation case (e.g. default build with no embed_llama
// tag, or $RECALL_EMBED_PROVIDER unset and no local backend). Callers
// should pass a nil embedder to recall.SearchHybrid — the engine falls
// back to BM25 cleanly.
//
// Returns (nil, err) for misconfiguration users should know about — e.g.
// RECALL_EMBED_PROVIDER=openai without OPENAI_API_KEY.
func (e *Engine) Embedder() (recall.Embedder, error) {
	e.embOnce.Do(func() {
		emb, err := recall.ResolveEmbedder()
		if err != nil {
			// Stub builds + no API provider = graceful fallback to BM25.
			if errors.Is(err, recall.ErrLocalEmbedderNotCompiled) &&
				recall.ResolveAPIProvider() == recall.ProviderLocal {
				return
			}
			e.embErr = err
			return
		}
		e.emb = emb
	})
	return e.emb, e.embErr
}

// SetEmbedder overrides the lazy-resolved embedder. Used by tests to
// inject a MockEmbedder without going through the env-driven factory.
// Must be called before the first [Embedder] call.
func (e *Engine) SetEmbedder(emb recall.Embedder) {
	e.embOnce.Do(func() { e.emb = emb })
}

// Close releases both the embedder (if constructed) and the recall
// engine. Safe to call even if neither was fully initialised.
func (e *Engine) Close() error {
	var embErr error
	if e.emb != nil {
		embErr = e.emb.Close()
	}
	rErr := e.rcl.Close()
	if embErr != nil {
		return embErr
	}
	return rErr
}
