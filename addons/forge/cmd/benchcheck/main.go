// Extended Tier-1/Tier-2 bench probe for golearn#141 / #139 KPI prototyping.
// Scratch harness in a detached worktree; the productised version is #139.
//
// KPIs measured per run:
//   - parse_valid: reply parsed as schema-valid JSON through the shipped adapter
//   - contract_valid: parsed AND content contract holds (4 choices, index in range)
//   - latency per executed request; throttles excluded from denominators
//   - sidecar (direct HTTP, labeled as such): prompt/completion tokens and,
//     on OpenRouter, the provider-reported cost per request
//   - guessability: choices-only probe over the questions this run generated
//
// Schema variants for the improvement loop:
//
//	base    — the strict schema used in Tier-1 so far
//	minmax  — adds "minItems":4,"maxItems":4 to choices (pre-registered H1:
//	          raises contract_valid without hurting parse_valid or latency)
package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
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

const schemaBase = `{"type":"object","additionalProperties":false,"properties":{"prompt":{"type":"string"},"choices":{"type":"array","items":{"type":"string"}},"correct_index":{"type":"integer"}},"required":["prompt","choices","correct_index"]}`
const schemaMinMax = `{"type":"object","additionalProperties":false,"properties":{"prompt":{"type":"string"},"choices":{"type":"array","minItems":4,"maxItems":4,"items":{"type":"string"}},"correct_index":{"type":"integer"}},"required":["prompt","choices","correct_index"]}`

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

// sidecar posts the same request shape the adapter sends, directly, to read
// the usage block the adapter does not parse. Measurement sidecar only —
// mechanism claims stay with the adapter path.
func sidecar(endpoint, key, model, schema string, includeCost bool) (promptTok, complTok int, cost float64, err error) {
	msgs := []map[string]string{
		{"role": "system", "content": systemMsg},
		{"role": "user", "content": userMsg},
	}
	body := map[string]any{
		"model":    model,
		"messages": msgs,
		"response_format": map[string]any{
			"type": "json_schema",
			"json_schema": map[string]any{
				"name": "forge_response", "strict": true, "schema": json.RawMessage(schema),
			},
		},
	}
	if includeCost {
		body["usage"] = map[string]any{"include": true}
	}
	raw, _ := json.Marshal(body)
	req, _ := http.NewRequest("POST", endpoint+"/chat/completions", bytes.NewReader(raw))
	req.Header.Set("Authorization", "Bearer "+key)
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: 3 * time.Minute}
	resp, err := client.Do(req)
	if err != nil {
		return 0, 0, 0, err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	var reply struct {
		Usage struct {
			PromptTokens     int     `json:"prompt_tokens"`
			CompletionTokens int     `json:"completion_tokens"`
			Cost             float64 `json:"cost"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(data, &reply); err != nil {
		return 0, 0, 0, err
	}
	return reply.Usage.PromptTokens, reply.Usage.CompletionTokens, reply.Usage.Cost, nil
}

func main() {
	profileID := "openrouter"
	model := "deepseek/deepseek-v4-flash-0731"
	n := 20
	variant := "base"
	if len(os.Args) > 1 {
		profileID = os.Args[1]
	}
	if len(os.Args) > 2 {
		model = os.Args[2]
	}
	if len(os.Args) > 3 {
		n, _ = strconv.Atoi(os.Args[3])
	}
	if len(os.Args) > 4 {
		variant = os.Args[4]
	}
	schema := schemaBase
	if variant == "minmax" {
		schema = schemaMinMax
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
	fmt.Printf("RUN profile=%s model=%s n=%d schema_variant=%s prompt_hash=%x sampling=provider-default\n",
		profileID, model, n, variant, promptHash[:8])

	parseOK, contractOK, executed, throttled := 0, 0, 0, 0
	var latencies []float64
	var questions []quiz
	for i := 0; executed < n && i < 2*n; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
		start := time.Now()
		var q quiz
		gerr := c.Generate(ctx, ports.Request{System: systemMsg, User: userMsg, Schema: []byte(schema)}, &q)
		cancel()
		el := time.Since(start).Seconds()
		if gerr != nil && strings.Contains(gerr.Error(), "HTTP 429") {
			throttled++
			fmt.Printf("GEN %02d throttled (excluded) err=%v\n", i+1, gerr)
			time.Sleep(7 * time.Second)
			continue
		}
		executed++
		latencies = append(latencies, el)
		pv := gerr == nil
		cv := pv && q.Prompt != "" && len(q.Choices) == 4 && q.Correct >= 0 && q.Correct < 4
		if pv {
			parseOK++
		}
		if cv {
			contractOK++
			questions = append(questions, q)
		}
		fmt.Printf("GEN %02d parse_valid=%v contract_valid=%v latency=%.2fs choices=%d err=%v\n",
			i+1, pv, cv, el, len(q.Choices), gerr)
		time.Sleep(7 * time.Second)
	}
	plo, phi := wilson(parseOK, executed)
	clo, chi := wilson(contractOK, executed)
	var sum float64
	for _, l := range latencies {
		sum += l
	}
	mean := 0.0
	if len(latencies) > 0 {
		mean = sum / float64(len(latencies))
	}
	fmt.Printf("PARSE  rate=%d/%d wilson95=[%.3f,%.3f]\n", parseOK, executed, plo, phi)
	fmt.Printf("CONTRACT rate=%d/%d wilson95=[%.3f,%.3f] throttled_excluded=%d mean_latency=%.2fs\n",
		contractOK, executed, clo, chi, throttled, mean)

	// Token/cost sidecar, m=3.
	tokP, tokC, costSum, m := 0, 0, 0.0, 0
	for i := 0; i < 3; i++ {
		p, cpl, cost, serr := sidecar(profile.DefaultEndpoint, key, model, schema, profileID == "openrouter")
		if serr != nil {
			fmt.Printf("SIDECAR %d err=%v\n", i+1, serr)
			continue
		}
		tokP += p
		tokC += cpl
		costSum += cost
		m++
		time.Sleep(7 * time.Second)
	}
	if m > 0 {
		fmt.Printf("USAGE sidecar m=%d mean_prompt_tokens=%d mean_completion_tokens=%d mean_reported_cost_usd=%.6f\n",
			m, tokP/m, tokC/m, costSum/float64(m))
	}

	// Guessability probe: choices only, no stem. Chance level 0.25.
	guessSchema := `{"type":"object","additionalProperties":false,"properties":{"choice_index":{"type":"integer"}},"required":["choice_index"]}`
	guessed, gN := 0, 0
	limit := len(questions)
	if limit > 8 {
		limit = 8
	}
	for _, q := range questions[:limit] {
		var choicesList strings.Builder
		for i, ch := range q.Choices {
			fmt.Fprintf(&choicesList, "%d: %s\n", i, ch)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		var g struct {
			ChoiceIndex int `json:"choice_index"`
		}
		gerr := c.Generate(ctx, ports.Request{
			System: "You are given only the answer choices of a hidden multiple-choice question. Pick the index most likely to be the correct answer.",
			User:   choicesList.String(),
			Schema: []byte(guessSchema),
		}, &g)
		cancel()
		if gerr != nil {
			continue
		}
		gN++
		if g.ChoiceIndex == q.Correct {
			guessed++
		}
		time.Sleep(7 * time.Second)
	}
	if gN > 0 {
		glo, ghi := wilson(guessed, gN)
		fmt.Printf("GUESSABILITY choices_only=%d/%d wilson95=[%.3f,%.3f] chance=0.25 (interval above chance flags leaking options)\n",
			guessed, gN, glo, ghi)
	}
}
