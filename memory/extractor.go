package memory

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"pedro/providers/openaiutil"
	"pedro/shared"

	"github.com/openai/openai-go"
)

const extractionSystemPrompt = `You are a memory extraction agent. Your ONLY job is to write long-term memories about the USER that will remain useful in future chats.

DEFAULT: DO NOT SAVE. When unsure, output [].

HARD GATE (must pass ALL):
A) The fact is about the USER (not assistant, not general info, not third parties unless directly relevant to user).
B) The user stated it as true about themselves (self-disclosure), not a question, not speculation.
C) It is stable for months/years OR a long-term preference/goal. If it could change within weeks, do not store.
D) It is likely useful for personalization later. If it's merely interesting, do not store.
E) It is specific and unambiguous.

DO NOT STORE (examples):
- The users questions, tasks, or one-off plans (“I am going to X today”)
- Ephemeral states (mood today, temporary location, current problem, purchases unless clearly enduring)
- Random favorites mentioned once (“I like pizza”) unless strongly emphasized as enduring (“my favorite food is…”)
- Technical content, instructions, code, or product opinions unless it's a stable preference/tool choice (“I always use zsh”, “I prefer OpenCode”)
- Sensitive data (exact address, passwords, API keys, health details) unless user explicitly asks to remember AND it is safe/minimal

QUALITY REQUIREMENTS:
1) Provide "confidence" 0.0-1.0. If < 0.8, do not store.
2) Do not create memories with importance 1-2. Only store importance 3-5.
3) Skip anything already present in Existing Memories unless it has changed.

KEYS/VALUES:
- key: short semantic snake_case (e.g., user_location_country, preferred_language)
- value: one sentence, no extra details, no speculation
- category: personal|preference|goal|technical

IMPORTANCE (3-5 only):
5: identity anchors (name, country/region, primary job/role)
4: strong enduring preferences, long-term goals, close relationships
3: stable habits/routines, consistently used tools/platforms

OUTPUT: Return ONLY a JSON array. If nothing qualifies, return [].

Schema:
[
  {
    "key": "short_semantic_key",
    "value": "concise durable fact",
    "category": "personal|preference|goal|technical",
    "importance": 3,
    "confidence": 0.0
  }
];
`

type ExtractedMemory struct {
	Key        string  `json:"key"`
	Value      string  `json:"value"`
	Category   string  `json:"category"`
	Importance int     `json:"importance"`
	Confidence float64 `json:"confidence"`
}

type Extractor struct {
	client openai.Client
	model  string
	store  shared.MemoryBackend
}

func NewExtractor(client openai.Client, model string, store shared.MemoryBackend) *Extractor {
	return &Extractor{client: client, model: model, store: store}
}

func (e *Extractor) ExtractAndSave(
	ctx context.Context,
	userContent string,
	assistantResponse string,
	conversationID int64,
) {
	if e.store == nil {
		return
	}

	existingMemories, _ := e.store.GetMemories()
	existingSection := formatExistingMemories(existingMemories)

	prompt := fmt.Sprintf(
		"Existing Memories:\n%s\n\n---\n\nConversation:\nUser: %s\n\nAssistant: %s\n\n---\n\nExtract durable personal facts from the user's message above. Return JSON array.",
		existingSection,
		userContent,
		assistantResponse,
	)

	response, err := openaiutil.ExtractCompletion(ctx, e.client, e.model, extractionSystemPrompt, prompt)
	if err != nil {
		log.Printf("[memory] extraction failed: %v", err)
		return
	}

	memories := parseExtractionResponse(response)
	for _, m := range memories {
		if m.Key == "" || m.Value == "" || m.Confidence < 0.8 {
			continue
		}
		importance := m.Importance
		if importance < 1 || importance > 5 {
			importance = 3
		}
		source := fmt.Sprintf("conversation:%d", conversationID)
		if err := e.store.SaveMemoryWithMeta(m.Key, m.Value, m.Category, source, importance); err != nil {
			log.Printf("[memory] failed to save extracted memory %q: %v", m.Key, err)
		}
	}
}

func formatExistingMemories(memories []shared.MemoryRecord) string {
	if len(memories) == 0 {
		return "(none)"
	}
	var b strings.Builder
	for _, m := range memories {
		fmt.Fprintf(&b, "- %s: %s (category: %s, importance: %d)\n", m.Key, m.Value, m.Category, m.Importance)
	}
	return b.String()
}

func parseExtractionResponse(response string) []ExtractedMemory {
	text := strings.TrimSpace(response)
	// Handle markdown code blocks
	if strings.HasPrefix(text, "```") {
		lines := strings.Split(text, "\n")
		var jsonLines []string
		inBlock := false
		for _, line := range lines {
			if strings.HasPrefix(line, "```") {
				inBlock = !inBlock
				continue
			}
			if inBlock {
				jsonLines = append(jsonLines, line)
			}
		}
		text = strings.TrimSpace(strings.Join(jsonLines, "\n"))
	}

	var memories []ExtractedMemory
	if err := json.Unmarshal([]byte(text), &memories); err != nil {
		log.Printf("[memory] failed to parse extraction response: %v (response: %s)", err, text)
		return nil
	}
	return memories
}
