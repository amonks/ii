// Command zone2terraform converts BIND zone files into Terraform
// aws_route53_zone and aws_route53_record resources.
//
// Based on tfz53 by Calle Pettersson (https://github.com/carlpett/tfz53),
// licensed under the Apache License, Version 2.0.
//
// Usage:
//
//	zone2terraform -dir aws/zones -out aws/terraform
//	zone2terraform -domain monks.co -zone-file zones/monks.co
package main

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

var (
	flagDomain   = flag.String("domain", "", "Name of domain (single-file mode)")
	flagZoneFile = flag.String("zone-file", "", "Path to zone file (single-file mode)")
	flagDir      = flag.String("dir", "", "Directory containing zone files (batch mode)")
	flagOut      = flag.String("out", "", "Output directory for generated .tf files (batch mode)")
)

type record struct {
	name    string
	typ     string
	ttl     string
	data    []string
	comment string
}

type recordKey struct {
	name string
	typ  string
}

func main() {
	flag.Parse()

	// Batch mode: process all zone files in a directory.
	if *flagDir != "" {
		if *flagOut == "" {
			log.Fatal("-out is required when using -dir")
		}
		if err := generateAll(*flagDir, *flagOut); err != nil {
			log.Fatal(err)
		}
		return
	}

	// Single-file mode.
	if *flagDomain == "" {
		log.Fatal("Domain is required (use -domain or -dir)")
	}
	if *flagZoneFile == "" {
		log.Fatal("Zone file is required")
	}
	fmt.Print(generateForZone(*flagDomain, *flagZoneFile))
}

// generateAll processes all zone files in zonesDir and writes .tf files to outDir.
func generateAll(zonesDir, outDir string) error {
	// Remove old generated files.
	matches, _ := filepath.Glob(filepath.Join(outDir, "generated_*.tf"))
	for _, m := range matches {
		if err := os.Remove(m); err != nil {
			return fmt.Errorf("removing %s: %w", m, err)
		}
	}

	entries, err := os.ReadDir(zonesDir)
	if err != nil {
		return fmt.Errorf("reading %s: %w", zonesDir, err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if info.Size() == 0 {
			continue
		}

		domain := entry.Name()
		zoneFile := filepath.Join(zonesDir, domain)
		output := generateForZone(domain, zoneFile)

		outPath := filepath.Join(outDir, "generated_"+domain+".tf")
		if err := os.WriteFile(outPath, []byte(output), 0644); err != nil {
			return fmt.Errorf("writing %s: %w", outPath, err)
		}
	}

	return nil
}

func generateForZone(domain, zoneFilePath string) string {
	f, err := os.Open(zoneFilePath)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()

	records := make(map[recordKey]*record)
	var order []recordKey
	var lastComment string

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			lastComment = ""
			continue
		}
		if after, ok := strings.CutPrefix(line, ";;"); ok {
			lastComment = strings.TrimSpace(after)
			continue
		}
		if after, ok := strings.CutPrefix(line, ";"); ok {
			lastComment = strings.TrimSpace(after)
			continue
		}

		parts := strings.Fields(line)
		if len(parts) < 4 {
			continue
		}
		// name ttl IN type data...
		name := parts[0]
		ttl := parts[1]
		// parts[2] is "IN"
		typ := parts[3]

		// Skip SOA and NS records.
		if typ == "SOA" || typ == "NS" {
			continue
		}

		data := extractData(line, typ)

		// Expand @ to domain.
		fqdn := name
		if fqdn == "@" {
			fqdn = domain + "."
		} else {
			fqdn = fqdn + "." + domain + "."
		}
		fqdn = strings.ToLower(fqdn)

		key := recordKey{fqdn, typ}
		if existing, ok := records[key]; ok {
			existing.data = append(existing.data, data)
		} else {
			r := &record{
				name:    fqdn,
				typ:     typ,
				ttl:     ttl,
				data:    []string{data},
				comment: lastComment,
			}
			records[key] = r
			order = append(order, key)
		}
		lastComment = ""
	}
	if err := scanner.Err(); err != nil {
		log.Fatal(err)
	}

	// Sort: reverse by "name-type" to match original tfz53 output.
	sort.Slice(order, func(i, j int) bool {
		ki := order[i].name + "-" + order[i].typ
		kj := order[j].name + "-" + order[j].typ
		return ki > kj
	})

	var b strings.Builder
	zoneID := strings.ReplaceAll(domain, ".", "-")
	fmt.Fprintf(&b, "resource \"aws_route53_zone\" %q {\n", zoneID)
	fmt.Fprintf(&b, "  name = %q\n", domain)
	fmt.Fprintf(&b, "}\n")

	for _, key := range order {
		r := records[key]
		resourceID := sanitizeName(r.name) + "-" + r.typ

		fmt.Fprintln(&b)
		if r.comment != "" {
			fmt.Fprintf(&b, "# %s\n", r.comment)
		}
		fmt.Fprintf(&b, "resource \"aws_route53_record\" %q {\n", resourceID)
		fmt.Fprintf(&b, "  zone_id = aws_route53_zone.%s.zone_id\n", zoneID)
		fmt.Fprintf(&b, "  name    = %q\n", r.name)
		fmt.Fprintf(&b, "  type    = %q\n", r.typ)
		fmt.Fprintf(&b, "  ttl     = %q\n", r.ttl)
		fmt.Fprintf(&b, "  records = [%s]\n", formatRecords(r.data))
		fmt.Fprintf(&b, "}\n")
	}
	return b.String()
}

// extractData pulls the record data from a zone file line.
// For TXT/SPF records, we strip the surrounding quotes so Terraform gets the raw value.
func extractData(line, typ string) string {
	parts := strings.Fields(line)
	raw := strings.Join(parts[4:], " ")

	if typ == "TXT" || typ == "SPF" {
		raw = strings.TrimSpace(raw)
		if len(raw) >= 2 && raw[0] == '"' && raw[len(raw)-1] == '"' {
			raw = raw[1 : len(raw)-1]
		}
	}

	if typ == "CNAME" {
		raw = strings.ToLower(raw)
	}

	return raw
}

func formatRecords(data []string) string {
	var parts []string
	for _, d := range data {
		parts = append(parts, fmt.Sprintf("%q", d))
	}
	return strings.Join(parts, ", ")
}

// sanitizeName converts a DNS FQDN into a Terraform-safe resource name component.
func sanitizeName(name string) string {
	name = strings.TrimRight(name, ".")
	name = strings.ReplaceAll(name, ".", "-")
	name = strings.ReplaceAll(name, "*", "wildcard")
	var b strings.Builder
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			b.WriteRune(r)
		} else {
			b.WriteRune('_')
		}
	}
	result := b.String()
	if len(result) > 0 && !((result[0] >= 'a' && result[0] <= 'z') || (result[0] >= 'A' && result[0] <= 'Z') || result[0] == '_') {
		result = "_" + result
	}
	return result
}
