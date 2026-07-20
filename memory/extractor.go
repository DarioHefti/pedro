package memory

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"github.com/openai/openai-go"
	"pedro/providers/openaiutil"
	"pedro/shared"
)

const extractionSystemPrompt = `You are a memory extraction agent. Your ONLY job is to extract durable personal facts from a conversation.

RULES:
1. ONLY extract facts that are:
   - Personal to the user (name, preferences, habits, goals, relationships, location, job, etc.)
   - Durable (will remain true across future conversations)
   - NOT general knowledge or opinions about topics
   - NOT temporary information (weather, current events, one-time requests)
2. NEVER extract:
   - What the user asked about (questions ≠ facts)
   - Technical explanations or how-to information
   - Opinions about technology, products, etc.
   - Anything that changes frequently
3. If a fact already exists in "Existing Memories" and hasn't changed, skip it.
4. Keep keys short and semantic (e.g., "user_name", "favorite_color", "job_title").
5. Keep values concise (one sentence max).
6. Assign importance 1-5:
   - 5: name, location, job (critical personal identity)
   - 4: preferences, goals, family (important context)
   - 3: habits, routines, tools used (useful context)
   - 2: minor details, passing mentions
   - 1: rarely useful information

OUTPUT: Return ONLY a JSON array. If nothing to extract, return [].

[
  {
    "key": "short_semantic_key",
    "value": "concise fact",
    "category": "personal|preference|goal|technical|other",
    "importance": 3
  }
]`

type ExtractedMemory struct {
	Key        string `json:"key"`
	Value      string `json:"value"`
	Category   string `json:"category"`
	Importance int    `json:"importance"`
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
		if m.Key == "" || m.Value == "" {
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
