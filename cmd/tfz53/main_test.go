package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGeneratedFilesMatchExpected(t *testing.T) {
	root := os.Getenv("MONKS_ROOT")
	if root == "" {
		t.Skip("MONKS_ROOT not set")
	}

	zonesDir := filepath.Join(root, "aws", "zones")
	terraformDir := filepath.Join(root, "aws", "terraform")

	entries, err := os.ReadDir(zonesDir)
	if err != nil {
		t.Fatal(err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		domainName := entry.Name()
		t.Run(domainName, func(t *testing.T) {
			zoneData, err := os.ReadFile(filepath.Join(zonesDir, domainName))
			if err != nil {
				t.Fatal(err)
			}
			if len(zoneData) == 0 {
				t.Skip("empty zone file")
			}

			expectedFile := filepath.Join(terraformDir, "generated_"+domainName+".tf")
			expected, err := os.ReadFile(expectedFile)
			if err != nil {
				t.Fatalf("reading expected output: %v", err)
			}

			got := generateForZone(domainName, filepath.Join(zonesDir, domainName))

			if got != string(expected) {
				t.Errorf("output mismatch for %s\n--- expected ---\n%s\n--- got ---\n%s", domainName, expected, got)
			}
		})
	}
}

func TestSanitizeName(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"monks.co.", "monks-co"},
		{"www.monks.co.", "www-monks-co"},
		{"*.brigid.ss.cx.", "wildcard-brigid-ss-cx"},
		{"_atproto.monks.co.", "_atproto-monks-co"},
		{"20240417205709pm._domainkey.monks.co.", "_20240417205709pm-_domainkey-monks-co"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := sanitizeName(tt.input)
			if got != tt.want {
				t.Errorf("sanitizeName(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestExtractData(t *testing.T) {
	tests := []struct {
		line string
		typ  string
		want string
	}{
		{"@ 300 IN A 37.16.29.186", "A", "37.16.29.186"},
		{"@ 300 IN CNAME example.com.", "CNAME", "example.com."},
		{`@ 300 IN TXT "v=spf1 include:spf.messagingengine.com ?all"`, "TXT", "v=spf1 include:spf.messagingengine.com ?all"},
		{`@ 300 IN SPF "v=spf1 include:spf.messagingengine.com -all"`, "SPF", "v=spf1 include:spf.messagingengine.com -all"},
		{"@ 300 IN MX 10 in1-smtp.messagingengine.com.", "MX", "10 in1-smtp.messagingengine.com."},
	}
	for _, tt := range tests {
		t.Run(tt.typ, func(t *testing.T) {
			got := extractData(tt.line, tt.typ)
			if got != tt.want {
				t.Errorf("extractData(%q, %q) = %q, want %q", tt.line, tt.typ, got, tt.want)
			}
		})
	}
}

func TestFormatRecords(t *testing.T) {
	got := formatRecords([]string{"37.16.29.186"})
	if got != `"37.16.29.186"` {
		t.Errorf("got %q", got)
	}

	got = formatRecords([]string{"10 in1-smtp.messagingengine.com.", "20 in2-smtp.messagingengine.com."})
	if !strings.Contains(got, `"10 in1-smtp.messagingengine.com."`) {
		t.Errorf("got %q", got)
	}
}
