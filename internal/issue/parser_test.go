package issue

import (
	"testing"
)

func TestParseURL_Valid(t *testing.T) {
	tests := []struct {
		url    string
		org    string
		repo   string
		number string
	}{
		{"https://github.com/wso2/product-apim/issues/4856", "wso2", "product-apim", "4856"},
		{"https://github.com/wso2/carbon-apimgt/issues/123", "wso2", "carbon-apimgt", "123"},
		{"http://github.com/org/repo/issues/1", "org", "repo", "1"},
	}

	for _, tt := range tests {
		t.Run(tt.url, func(t *testing.T) {
			iss, err := ParseURL(tt.url)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if iss.Org != tt.org {
				t.Errorf("org: got %q, want %q", iss.Org, tt.org)
			}
			if iss.Repo != tt.repo {
				t.Errorf("repo: got %q, want %q", iss.Repo, tt.repo)
			}
			if iss.Number != tt.number {
				t.Errorf("number: got %q, want %q", iss.Number, tt.number)
			}
		})
	}
}

func TestParseURL_Invalid(t *testing.T) {
	tests := []string{
		"not-a-url",
		"https://github.com/wso2/product-apim",
		"https://github.com/wso2/product-apim/pulls/123",
		"https://gitlab.com/org/repo/issues/1",
		"",
	}

	for _, url := range tests {
		t.Run(url, func(t *testing.T) {
			_, err := ParseURL(url)
			if err == nil {
				t.Error("expected error, got nil")
			}
		})
	}
}
