package issue

import (
	"fmt"
	"regexp"
)

type Issue struct {
	URL    string
	Org    string
	Repo   string
	Number string
}

var issueURLPattern = regexp.MustCompile(`^https?://github\.com/([^/]+)/([^/]+)/issues/(\d+)`)

func ParseURL(url string) (*Issue, error) {
	matches := issueURLPattern.FindStringSubmatch(url)
	if matches == nil {
		return nil, fmt.Errorf("invalid GitHub issue URL: %s (expected: https://github.com/<org>/<repo>/issues/<number>)", url)
	}

	return &Issue{
		URL:    url,
		Org:    matches[1],
		Repo:   matches[2],
		Number: matches[3],
	}, nil
}
