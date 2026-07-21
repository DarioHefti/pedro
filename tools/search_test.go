package tools

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestExtractDDGURL(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "DDG redirect with encoded URL",
			input:    "//duckduckgo.com/l/?uddg=https%3A%2F%2Fexample.com&rut=something",
			expected: "https://example.com",
		},
		{
			name:     "DDG redirect with complex encoded URL",
			input:    "https://duckduckgo.com/l/?uddg=https%3A%2F%2Fexample.com%2Fpath%3Fq%3D123&rut=something",
			expected: "https://example.com/path?q=123",
		},
		{
			name:     "Direct URL passthrough",
			input:    "https://example.com",
			expected: "https://example.com",
		},
		{
			name:     "Protocol-relative direct URL",
			input:    "//example.com",
			expected: "//example.com",
		},
		{
			name:     "Empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "Non-DDG redirect URL",
			input:    "https://google.com/l/?uddg=test",
			expected: "https://google.com/l/?uddg=test",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractDDGURL(tt.input)
			if result != tt.expected {
				t.Errorf("extractDDGURL(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestOpenAlexResponseParsing(t *testing.T) {
	mockResponse := openalexResponse{}
	mockResponse.Meta.Count = 1
	mockResponse.Results = []openalexWork{
		{
			ID:              "https://openalex.org/W1234567890",
			DOI:             "https://doi.org/10.1234/test",
			DisplayName:     "Test Paper Title",
			PublicationYear: 2024,
			CitedByCount:    42,
			OpenAccess: struct {
				IsOA    bool   `json:"is_oa"`
				OAURL   string `json:"oa_url"`
				OAStatus string `json:"oa_status"`
			}{
				IsOA:    true,
				OAURL:   "https://example.com/paper.pdf",
				OAStatus: "gold",
			},
			PrimaryLocation: struct {
				Source struct {
					DisplayName string `json:"display_name"`
				} `json:"source"`
			}{
				Source: struct {
					DisplayName string `json:"display_name"`
				}{
					DisplayName: "Test Journal",
				},
			},
		},
	}

	jsonData, err := json.Marshal(mockResponse)
	if err != nil {
		t.Fatalf("Failed to marshal mock response: %v", err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(jsonData)
	}))
	defer server.Close()

	// Test that we can parse the response correctly
	var parsed openalexResponse
	if err := json.Unmarshal(jsonData, &parsed); err != nil {
		t.Fatalf("Failed to parse OpenAlex response: %v", err)
	}

	if len(parsed.Results) != 1 {
		t.Fatalf("Expected 1 result, got %d", len(parsed.Results))
	}

	work := parsed.Results[0]
	if work.DisplayName != "Test Paper Title" {
		t.Errorf("Expected display_name 'Test Paper Title', got %q", work.DisplayName)
	}
	if work.DOI != "https://doi.org/10.1234/test" {
		t.Errorf("Expected DOI 'https://doi.org/10.1234/test', got %q", work.DOI)
	}
	if work.PublicationYear != 2024 {
		t.Errorf("Expected publication_year 2024, got %d", work.PublicationYear)
	}
	if work.CitedByCount != 42 {
		t.Errorf("Expected cited_by_count 42, got %d", work.CitedByCount)
	}
	if !work.OpenAccess.IsOA {
		t.Error("Expected is_oa to be true")
	}
	if work.PrimaryLocation.Source.DisplayName != "Test Journal" {
		t.Errorf("Expected source display_name 'Test Journal', got %q", work.PrimaryLocation.Source.DisplayName)
	}
}
