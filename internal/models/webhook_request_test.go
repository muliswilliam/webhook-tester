package models

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/datatypes"
)

func TestCapturedValues(t *testing.T) {
	got := CapturedValues(map[string][]string{"A": {"1"}, "B": {"1", "2"}, "C": {""}})
	assert.Equal(t, datatypes.JSONMap{"A": "1", "B": []string{"1", "2"}, "C": ""}, got)
}

func TestFieldValues(t *testing.T) {
	for name, tc := range map[string]struct {
		value any
		want  []string
	}{
		"string":                     {value: "a,b", want: []string{"a,b"}},
		"list":                       {value: []string{"a", "b"}, want: []string{"a", "b"}},
		"list read back from JSON":   {value: []any{"a", "b"}, want: []string{"a", "b"}},
		"number":                     {value: 42.5, want: []string{"42.5"}},
		"bool":                       {value: true, want: []string{"true"}},
		"null":                       {value: nil, want: []string{""}},
		"list of mixed values":       {value: []any{"a", 1.0, nil}, want: []string{"a", "1", ""}},
		"object, only hand-editable": {value: map[string]any{"k": "v"}, want: []string{"map[k:v]"}},
	} {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, tc.want, FieldValues(tc.value))
		})
	}
	assert.Equal(t, "a,b", FieldValue([]any{"a", "b"}), "displayed values are comma-joined")
}

func TestWebhookRequest_HeaderValues(t *testing.T) {
	wr := WebhookRequest{Headers: datatypes.JSONMap{
		"X-Multi":  []any{"one", "two, three"},
		"X-Joined": "a,b", // captured before repeated headers were kept
		"X-Number": 5.0,
	}}
	assert.Equal(t, http.Header{
		"X-Multi":  {"one", "two, three"},
		"X-Joined": {"a,b"},
		"X-Number": {"5"},
	}, wr.HeaderValues())
}

func TestWebhookRequest_URLAt(t *testing.T) {
	for name, tc := range map[string]struct {
		base string
		wr   WebhookRequest
		want string
	}{
		"no subpath":              {base: "https://api.example.com/hooks", want: "https://api.example.com/hooks"},
		"subpath":                 {base: "https://api.example.com/hooks", wr: WebhookRequest{Path: "/orders/42"}, want: "https://api.example.com/hooks/orders/42"},
		"subpath, trailing slash": {base: "https://api.example.com/hooks/", wr: WebhookRequest{Path: "/orders/42"}, want: "https://api.example.com/hooks/orders/42"},
		"root subpath":            {base: "https://api.example.com/hooks", wr: WebhookRequest{Path: "/"}, want: "https://api.example.com/hooks/"},
		"bare host":               {base: "https://api.example.com", wr: WebhookRequest{Path: "/orders"}, want: "https://api.example.com/orders"},
		"escaped slash kept":      {base: "https://api.example.com/hooks", wr: WebhookRequest{Path: "/files/a%2Fb"}, want: "https://api.example.com/hooks/files/a%2Fb"},
		"escapes kept as sent":    {base: "https://api.example.com/hooks", wr: WebhookRequest{Path: "/a%20b/%7Ec"}, want: "https://api.example.com/hooks/a%20b/%7Ec"},
		"escaped base path kept":  {base: "https://api.example.com/my%2Fhooks", wr: WebhookRequest{Path: "/x"}, want: "https://api.example.com/my%2Fhooks/x"},
		"older decoded path":      {base: "https://api.example.com/hooks", wr: WebhookRequest{Path: "/a b"}, want: "https://api.example.com/hooks/a%20b"},
		"older decoded path with a lone %": {
			base: "https://api.example.com/hooks", wr: WebhookRequest{Path: "/100%"}, want: "https://api.example.com/hooks/100%25",
		},
		"raw query": {
			base: "https://api.example.com/hooks", wr: WebhookRequest{RawQuery: "b=x%20y&a=1&a=2"}, want: "https://api.example.com/hooks?b=x%20y&a=1&a=2",
		},
		"raw query after an encoded base query": {
			base: "https://api.example.com/hooks?sig=a%2Bb&z=1&a=0", wr: WebhookRequest{RawQuery: "b=2&a=1"},
			want: "https://api.example.com/hooks?sig=a%2Bb&z=1&a=0&b=2&a=1",
		},
		"base query untouched": {base: "https://api.example.com/hooks?b=2&a=1", want: "https://api.example.com/hooks?b=2&a=1"},
		"raw query preferred over the query map": {
			base: "https://api.example.com/hooks", wr: WebhookRequest{RawQuery: "a=1&a=2", Query: datatypes.JSONMap{"a": []any{"1", "2"}}},
			want: "https://api.example.com/hooks?a=1&a=2",
		},
		"older row: query map, encoded": {
			base: "https://api.example.com/hooks?token=abc", wr: WebhookRequest{Query: datatypes.JSONMap{"x": "1", "y": "a b"}},
			want: "https://api.example.com/hooks?token=abc&x=1&y=a+b",
		},
		"older row: non-string query value": {
			base: "https://api.example.com/hooks", wr: WebhookRequest{Query: datatypes.JSONMap{"n": 5.0}},
			want: "https://api.example.com/hooks?n=5",
		},
		"dots that aren't dot segments kept": {
			base: "https://api.example.com/hooks", wr: WebhookRequest{Path: "/v1..2/./.well-known/..x/%2e/a;b=.."},
			want: "https://api.example.com/hooks/v1..2/./.well-known/..x/%2e/a;b=..",
		},
		"empty segments kept": {base: "https://api.example.com/hooks", wr: WebhookRequest{Path: "/a//b/"}, want: "https://api.example.com/hooks/a//b/"},
		"subpath and query": {
			base: "https://api.example.com/hooks?k=v", wr: WebhookRequest{Path: "/a%2Fb", RawQuery: "x=1"},
			want: "https://api.example.com/hooks/a%2Fb?k=v&x=1",
		},
	} {
		t.Run(name, func(t *testing.T) {
			got, err := tc.wr.URLAt(tc.base)
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}

	_, err := WebhookRequest{}.URLAt("http://[::1")
	assert.Error(t, err)
}

// A subpath that any server could read as holding a ".." segment is refused
// rather than rewritten, since servers disagree on what one is.
func TestWebhookRequest_URLAtRefusesSubpathsThatCouldLeaveBase(t *testing.T) {
	for _, p := range []string{
		"/..",
		"/../admin",
		"/a/b/../../../admin",
		"/%2e%2e/admin",
		"/%2E./admin",
		"/..%2fadmin",
		"/..%2F..%2Fadmin",
		"/%2e%2e%2f%2e%2e%2fadmin",
		"/..%5c..%5cadmin",
		"/..\\admin",
		"/a\\..\\..\\admin",
		"/..;/..;/admin",
		"/..;jsessionid=x/admin",
		"/%2e%2e;x/admin",
		"/%252e%252e/admin",
		"/%25252e%25252e%25252fadmin",
	} {
		t.Run(p, func(t *testing.T) {
			_, err := WebhookRequest{Path: p}.URLAt("https://api.example.com/hooks/stripe")
			require.ErrorIs(t, err, ErrSubpathLeavesBase)
			assert.Contains(t, err.Error(), fmt.Sprintf("%q", p))
		})
	}
}
