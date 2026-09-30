package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/dezeat/golearn/addons/forge/internal/adapters/provider"
	"github.com/dezeat/golearn/addons/forge/internal/domain"
	"github.com/dezeat/golearn/addons/forge/internal/ports"
)

type quiz struct {
	Prompt  string   `json:"prompt"`
	Choices []string `json:"choices"`
	Correct int      `json:"correct_index"`
}

const schema = `{"type":"object","properties":{"prompt":{"type":"string"},"choices":{"type":"array","items":{"type":"string"}},"correct_index":{"type":"integer"}},"required":["prompt","choices","correct_index"]}`

func main() {
	endpoint := os.Getenv("FORGE_LIVE_ENDPOINT")
	if endpoint == "" {
		fmt.Println("FORGE_LIVE_ENDPOINT unset; skipping")
		return
	}
	models := os.Args[1:]
	profile, _ := domain.ProfileByID(domain.ProfileOllama)

	// Probe
	p0 := provider.NewOllama(profile, "", provider.WithEndpoint(endpoint))
	pr, err := p0.Probe(context.Background())
	fmt.Printf("PROBE reachable=%v authenticated=%v models=%d err=%v\n", pr.Reachable, pr.Authenticated, len(pr.Models), err)

	for _, m := range models {
		c := provider.NewOllama(profile, m, provider.WithEndpoint(endpoint),
			provider.WithEmbeddingModel("nomic-embed-text"))

		ctx, cancel := context.WithTimeout(context.Background(), 6*time.Minute)
		start := time.Now()
		var q quiz
		err := c.Generate(ctx, ports.Request{
			System: "You write multiple-choice questions. Reply with JSON only.",
			User:   "Write one multiple-choice question about Go goroutines with exactly 4 choices.",
			Schema: []byte(schema),
		}, &q)
		cancel()
		elapsed := time.Since(start)
		valid := err == nil && q.Prompt != "" && len(q.Choices) == 4 && q.Correct >= 0 && q.Correct < 4
		blob, _ := json.Marshal(q)
		fmt.Printf("GEN model=%s structured_valid=%v latency=%.1fs choices=%d err=%v\n",
			m, valid, elapsed.Seconds(), len(q.Choices), err)
		if len(blob) > 0 && valid {
			fmt.Printf("    sample_len=%d correct_index=%d\n", len(blob), q.Correct)
		}
	}

	// Embeddings
	ec := provider.NewOllama(profile, "qwen3:4b", provider.WithEndpoint(endpoint),
		provider.WithEmbeddingModel("nomic-embed-text"))
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	start := time.Now()
	vecs, err := ec.Embed(ctx, []string{"goroutines are lightweight threads", "channels synchronize goroutines"})
	if err != nil {
		fmt.Printf("EMBED err=%v\n", err)
	} else {
		sim, cerr := domain.Cosine(vecs[0], vecs[1])
		fmt.Printf("EMBED n=%d dim=%d cosine=%.4f bytes_per_vector=%d latency=%.2fs cosineErr=%v\n",
			len(vecs), vecs[0].Dim(), sim, len(domain.MarshalVector(vecs[0])), time.Since(start).Seconds(), cerr)
	}

	// Cancellation
	cctx, ccancel := context.WithCancel(context.Background())
	go func() { time.Sleep(2 * time.Second); ccancel() }()
	cstart := time.Now()
	var q2 quiz
	cerr := provider.NewOllama(profile, models[0], provider.WithEndpoint(endpoint)).
		Generate(cctx, ports.Request{User: "Write a very long essay about concurrency.", Schema: []byte(schema)}, &q2)
	fmt.Printf("CANCEL latency=%.3fs err=%v\n", time.Since(cstart).Seconds(), cerr)
}
