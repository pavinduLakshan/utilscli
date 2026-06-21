package main

import (
	"fmt"
	"strings"
)

// routePrompt makes common requests fast, private, and deterministic. It only
// returns an error when its confidence is too low; run then asks Claude Code.
func routePrompt(prompt string) (string, []string, error) {
	p := strings.TrimSpace(prompt)
	lower := strings.ToLower(p)
	type rule struct {
		command  string
		prefixes []string
	}
	rules := []rule{
		{"b64-decode", []string{"base64 decode ", "decode base64 ", "decode b64 "}},
		{"b64-encode", []string{"base64 encode ", "encode base64 ", "base64 ", "b64 "}},
		{"b64url-decode", []string{"base64url decode ", "base64 url decode ", "decode base64url "}},
		{"b64url-encode", []string{"base64url encode ", "base64 url encode ", "encode base64url "}},
		{"url-decode", []string{"url decode ", "decode url ", "decode this url "}},
		{"url-encode", []string{"url encode ", "encode url ", "encode this url "}},
		{"html-decode", []string{"html decode ", "decode html "}},
		{"html-encode", []string{"html encode ", "encode html "}},
		{"json-pretty", []string{"pretty json ", "beautify json ", "format json "}},
		{"json-minify", []string{"minify json "}},
		{"xml-pretty", []string{"pretty xml ", "beautify xml ", "format xml "}},
		{"xml-minify", []string{"minify xml "}},
		{"jwt", []string{"decode jwt ", "jwt decode ", "jwt "}},
		{"saml", []string{"decode saml ", "saml decode ", "saml "}},
		{"hash", []string{"hash ", "generate hashes for "}},
		{"timestamp", []string{"timestamp ", "convert timestamp ", "convert date "}},
		{"http", []string{"http status ", "status code "}},
	}
	for _, r := range rules {
		for _, prefix := range r.prefixes {
			if strings.HasPrefix(lower, prefix) {
				value := strings.TrimSpace(p[len(prefix):])
				if value == "" {
					return "", nil, fmt.Errorf("%s needs input", r.command)
				}
				return r.command, []string{value}, nil
			}
		}
	}
	if lower == "uuid" || lower == "generate uuid" || lower == "generate a uuid" {
		return "uuid", []string{"1"}, nil
	}
	if lower == "password" || strings.HasPrefix(lower, "generate password") {
		return "password", nil, nil
	}
	return "", nil, fmt.Errorf("I couldn't identify a utility for %q; run 'uc --help' for commands", prompt)
}
