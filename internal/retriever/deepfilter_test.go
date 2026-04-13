package retriever

import (
	"testing"
)

func TestParseIndices(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		max  int
		want []int
	}{
		{"simple", "0,2,5", 10, []int{0, 2, 5}},
		{"with spaces", " 1 , 3 , 7 ", 10, []int{1, 3, 7}},
		{"out of range dropped", "0,2,99", 10, []int{0, 2}},
		{"duplicates dropped", "1,1,3,3", 10, []int{1, 3}},
		{"empty string", "", 10, nil},
		{"negative dropped", "-1,2,5", 10, []int{2, 5}},
		{"garbage mixed in", "0,abc,3,!@#,5", 10, []int{0, 3, 5}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := parseIndices(tc.raw, tc.max)
			if len(got) != len(tc.want) {
				t.Errorf("parseIndices(%q, %d) = %v, want %v", tc.raw, tc.max, got, tc.want)
				return
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Errorf("parseIndices(%q, %d)[%d] = %d, want %d", tc.raw, tc.max, i, got[i], tc.want[i])
				}
			}
		})
	}
}

func TestDeepFilterSmallInput(t *testing.T) {
	// 10 or fewer chunks: DeepFilter should return them as-is, no LLM call.
	chunks := make([]Chunk, 8)
	for i := range chunks {
		chunks[i] = Chunk{DocID: string(rune('a' + i)), Score: 0.5}
	}
	// <=10 chunks: returns as-is without calling LLM (nil is safe here).
	result := DeepFilter(nil, chunks, "question", nil)
	if len(result) != 8 {
		t.Errorf("expected 8 chunks returned as-is, got %d", len(result))
	}
}

func TestDeepFilterNilLLM(t *testing.T) {
	chunks := make([]Chunk, 15)
	for i := range chunks {
		chunks[i] = Chunk{DocID: string(rune('a' + i)), Score: 0.5}
	}
	result := DeepFilter(nil, chunks, "question", nil)
	if len(result) != 15 {
		t.Errorf("nil llmCall should return all chunks, got %d", len(result))
	}
}
