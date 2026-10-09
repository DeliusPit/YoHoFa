package main

import "testing"

func TestDocumentAIChatURL(t *testing.T) {
	tests := map[string]string{
		"http://ai:8080":                     "http://ai:8080/v1/chat/completions",
		"http://ai:8080/":                    "http://ai:8080/v1/chat/completions",
		"http://ai:8080/v1":                  "http://ai:8080/v1/chat/completions",
		"http://ai:8080/v1/models":           "http://ai:8080/v1/chat/completions",
		"http://ai:8080/v1/chat/completions": "http://ai:8080/v1/chat/completions",
	}
	for input, want := range tests {
		got, err := documentAIChatURL(input)
		if err != nil {
			t.Fatalf("documentAIChatURL(%q) returned error: %v", input, err)
		}
		if got != want {
			t.Fatalf("documentAIChatURL(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestFinancialObjectIDsFromAnalysis(t *testing.T) {
	valid := map[int64]bool{2: true, 5: true, 8: true}
	got := financialObjectIDsFromAnalysis(`{"invoiceId":5,"financialObjectIds":[2,"5",99],"objects":[{"id":8},{"id":2}]}`, valid)
	want := []int64{2, 5, 8}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
}
