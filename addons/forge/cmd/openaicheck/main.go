// Tier-1 mechanism probe for golearn#141 — OpenAI lane.
// Scratch harness, run from a detached worktree; the productised version is #139.
package main

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"
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

// Strict-mode-compliant schema: OpenAI json_schema strict:true requires
// additionalProperties:false and every property listed in required.
const schema = `{"type":"object","additionalProperties":false,"properties":{"prompt":{"type":"string"},"choices":{"type":"array","items":{"type":"string"}},"correct_index":{"type":"integer"}},"required":["prompt","choices","correct_index"]}`

const systemMsg = "You write multiple-choice questions. Reply with JSON only."
const userMsg = "Write one multiple-choice question about Go goroutines with exactly 4 choices."

func wilson(k, n int) (lo, hi float64) {
	if n == 0 {
		return 0, 1
	}
	const z = 1.959963985
	p := float64(k) / float64(n)
	nf := float64(n)
	denom := 1 + z*z/nf
	center := (p + z*z/(2*nf)) / denom
	half := z * math.Sqrt(p*(1-p)/nf+z*z/(4*nf*nf)) / denom
	return center - half, center + half
}

func main() {
	profileID := "openai"
	model := "gpt-5-nano"
	n := 20
	if len(os.Args) > 1 {
		profileID = os.Args[1]
	}
	if len(os.Args) > 2 {
		model = os.Args[2]
	}
	if len(os.Args) > 3 {
		n, _ = strconv.Atoi(os.Args[3])
	}

	profile, perr := domain.ProfileByID(domain.ProfileID(profileID))
	if perr != nil {
		fmt.Println("unknown profile:", perr)
		os.Exit(1)
	}
	key := os.Getenv(profile.CredentialEnvVar)
	if key == "" {
		fmt.Printf("%s unset; aborting\n", profile.CredentialEnvVar)
		os.Exit(1)
	}
	secret := domain.NewSecret(key, domain.OriginEnvironment)
	c := provider.NewOpenAICompatible(profile, model, secret)

	promptHash := sha256.Sum256([]byte(systemMsg + "\x00" + userMsg + "\x00" + schema))
	fmt.Printf("RUN profile=%s model=%s n=%d prompt_hash=%x sampling=provider-default max_tokens=omitted reasoning=adapter-sends-no-control\n",
		profileID, model, n, promptHash[:8])

	// Probe: reachability + auth through the shipped adapter.
	pctx, pcancel := context.WithTimeout(context.Background(), 30*time.Second)
	pr, err := c.Probe(pctx)
	pcancel()
	fmt.Printf("PROBE reachable=%v authenticated=%v models=%d err=%v\n", pr.Reachable, pr.Authenticated, len(pr.Models), err)

	// structured_valid_rate over n executed requests. Amendment (recorded):
	// requests are paced below the account's RPM cap, and a 429 (throttle or
	// quota) never reached the model, so it is counted but excluded from the
	// validity denominator — the measurand is wire-format validity of
	// executed requests, not the provider's admission control.
	valid, executed, throttled := 0, 0, 0
	var latencies []float64
	for i := 0; executed < n && i < 2*n; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
		start := time.Now()
		var q quiz
		gerr := c.Generate(ctx, ports.Request{System: systemMsg, User: userMsg, Schema: []byte(schema)}, &q)
		cancel()
		el := time.Since(start).Seconds()
		if gerr != nil && strings.Contains(gerr.Error(), "HTTP 429") {
			throttled++
			fmt.Printf("GEN %02d throttled (excluded) latency=%.2fs err=%v\n", i+1, el, gerr)
			time.Sleep(7 * time.Second)
			continue
		}
		executed++
		latencies = append(latencies, el)
		ok := gerr == nil && q.Prompt != "" && len(q.Choices) == 4 && q.Correct >= 0 && q.Correct < 4
		if ok {
			valid++
		}
		fmt.Printf("GEN %02d structured_valid=%v latency=%.2fs choices=%d err=%v\n", i+1, ok, el, len(q.Choices), gerr)
		time.Sleep(7 * time.Second)
	}
	lo, hi := wilson(valid, executed)
	rate := 0.0
	if executed > 0 {
		rate = float64(valid) / float64(executed)
	}
	verdict := "FAIL"
	switch {
	case rate >= 0.90 && lo > 0.80:
		verdict = "PASS"
	case hi > 0.80 && (rate < 0.90 || lo <= 0.80):
		verdict = "UNDECIDED"
	}
	var sum float64
	for _, l := range latencies {
		sum += l
	}
	mean := 0.0
	if len(latencies) > 0 {
		mean = sum / float64(len(latencies))
	}
	fmt.Printf("VALID rate=%d/%d=%.3f throttled_excluded=%d wilson95=[%.3f,%.3f] floor=0.80 target=0.90 verdict=%s mean_latency=%.2fs\n",
		valid, executed, rate, throttled, lo, hi, verdict, mean)

	// Timeout: an undersized deadline must surface as a deadline error, not a hang.
	tctx, tcancel := context.WithTimeout(context.Background(), 1*time.Second)
	tstart := time.Now()
	var qt quiz
	terr := c.Generate(tctx, ports.Request{System: systemMsg, User: userMsg, Schema: []byte(schema)}, &qt)
	tcancel()
	fmt.Printf("TIMEOUT elapsed=%.2fs deadline_exceeded=%v err=%v\n",
		time.Since(tstart).Seconds(), errors.Is(terr, context.DeadlineExceeded), terr)

	// Cancel: release must follow the cancel signal promptly (<= 2s budget).
	cctx, ccancel := context.WithCancel(context.Background())
	go func() { time.Sleep(500 * time.Millisecond); ccancel() }()
	cstart := time.Now()
	var qc quiz
	cerr := c.Generate(cctx, ports.Request{System: systemMsg, User: userMsg, Schema: []byte(schema)}, &qc)
	release := time.Since(cstart).Seconds() - 0.5
	fmt.Printf("CANCEL release_after_cancel=%.3fs within_2s=%v canceled=%v err=%v\n",
		release, release <= 2.0, errors.Is(cerr, context.Canceled), cerr)

	// Usage capture: the adapter's response struct is the authority.
	var probeReply map[string]any
	_ = json.Unmarshal([]byte(`{}`), &probeReply)
	fmt.Println("USAGE adapter_parses_usage=false (openAIResponse has no usage field; cost_per_pack has no data path)")
}
