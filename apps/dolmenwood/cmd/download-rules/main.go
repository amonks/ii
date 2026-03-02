package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"golang.org/x/net/html"
)

const baseURL = "https://www.dolmenwood.necroticgnome.com/rules/doku.php?id="

type Section struct {
	Dir   string
	Pages []Page
}

type Page struct {
	ID    string
	Title string
}

var sections = []Section{
	{
		Dir: "starting-play",
		Pages: []Page{
			{"terminology", "Terminology"},
			{"character_statistics", "Character Statistics"},
			{"creating_a_character", "Creating a Character"},
			{"ability_scores", "Ability Scores"},
			{"alignment", "Alignment"},
			{"advancement", "Advancement"},
			{"languages", "Languages"},
		},
	},
	{
		Dir: "kindreds",
		Pages: []Page{
			{"mortals_fairies_and_demi-fey", "Mortals, Fairies, and Demi-Fey"},
			{"breggle", "Breggle"},
			{"elf", "Elf"},
			{"grimalkin", "Grimalkin"},
			{"human", "Human"},
			{"mossling", "Mossling"},
			{"woodgrue", "Woodgrue"},
		},
	},
	{
		Dir: "classes",
		Pages: []Page{
			{"bard", "Bard"},
			{"cleric", "Cleric"},
			{"enchanter", "Enchanter"},
			{"fighter", "Fighter"},
			{"friar", "Friar"},
			{"hunter", "Hunter"},
			{"knight", "Knight"},
			{"magician", "Magician"},
			{"thief", "Thief"},
		},
	},
	{
		Dir: "magic",
		Pages: []Page{
			{"arcane_magic", "Arcane Magic"},
			{"fairy_magic", "Fairy Magic"},
			{"holy_magic", "Holy Magic"},
			{"mossling_knacks", "Mossling Knacks"},
		},
	},
	{
		Dir: "equipment",
		Pages: []Page{
			{"adventuring_gear", "Adventuring Gear"},
			{"armour_and_weapons", "Armour and Weapons"},
			{"horses_and_vehicles", "Horses and Vehicles"},
			{"hounds", "Hounds"},
			{"lodgings_and_food", "Lodgings and Food"},
			{"beverages", "Beverages"},
			{"pipeleaf", "Pipeleaf"},
			{"common_fungi_and_herbs", "Common Fungi and Herbs"},
			{"specialist_services", "Specialist Services"},
			{"retainers", "Retainers"},
		},
	},
	{
		Dir: "adventuring",
		Pages: []Page{
			{"core_rules", "Core Rules"},
			{"time_and_movement", "Time and Movement"},
			{"encumbrance", "Encumbrance"},
			{"hazards_and_challenges", "Hazards and Challenges"},
			{"travel", "Travel"},
			{"camping", "Camping"},
			{"settlements", "Settlements"},
			{"dungeons", "Dungeons"},
			{"encounters", "Encounters"},
			{"combat", "Combat"},
			{"other_combat_matters", "Other Combat Matters"},
		},
	},
	{
		Dir: "appendices",
		Pages: []Page{
			{"breggle_kindred-class", "Breggle Kindred-Class"},
			{"elf_kindred-class", "Elf Kindred-Class"},
			{"grimalkin_kindred-class", "Grimalkin Kindred-Class"},
			{"mossling_kindred-class", "Mossling Kindred-Class"},
			{"woodgrue_kindred-class", "Woodgrue Kindred-Class"},
		},
	},
}

func main() {
	outDir := "apps/dolmenwood/rules"

	// If a page ID is given as an argument, just scrape that one page to stdout.
	if len(os.Args) > 1 {
		pageID := os.Args[1]
		md, err := fetchAndConvert(pageID)
		if err != nil {
			log.Fatal(err)
		}
		fmt.Print(md)
		return
	}

	for _, section := range sections {
		dir := filepath.Join(outDir, section.Dir)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			log.Fatalf("creating directory %s: %v", dir, err)
		}
		for _, page := range section.Pages {
			fmt.Printf("Fetching %s/%s...\n", section.Dir, page.ID)
			md, err := fetchAndConvert(page.ID)
			if err != nil {
				log.Printf("  ERROR: %v", err)
				continue
			}
			filename := page.ID + ".md"
			path := filepath.Join(dir, filename)
			if err := os.WriteFile(path, []byte(md), 0o644); err != nil {
				log.Printf("  ERROR writing %s: %v", path, err)
				continue
			}
			fmt.Printf("  -> %s\n", path)
		}
	}
	fmt.Println("Done.")
}

func fetchAndConvert(pageID string) (string, error) {
	url := baseURL + pageID
	resp, err := http.Get(url)
	if err != nil {
		return "", fmt.Errorf("fetching %s: %w", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("status %d for %s", resp.StatusCode, url)
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return "", fmt.Errorf("parsing HTML: %w", err)
	}

	// The content is inside <div class="page group"> within <main id="dokuwiki__content">
	content := doc.Find("#dokuwiki__content .page.group")

	// Remove the TOC
	content.Find("#dw__toc").Remove()

	// Remove section edit buttons
	content.Find(".secedit").Remove()

	var b strings.Builder
	convertNode(&b, content, 0)
	return strings.TrimSpace(b.String()) + "\n", nil
}

func convertNode(b *strings.Builder, sel *goquery.Selection, depth int) {
	sel.Contents().Each(func(i int, s *goquery.Selection) {
		node := s.Get(0)
		if node == nil {
			return
		}

		switch node.Type {
		case html.ElementNode:
			tag := goquery.NodeName(s)
			switch tag {
			case "h1":
				text := strings.TrimSpace(s.Text())
				b.WriteString("# " + text + "\n\n")
			case "h2":
				text := strings.TrimSpace(s.Text())
				b.WriteString("## " + text + "\n\n")
			case "h3":
				text := strings.TrimSpace(s.Text())
				b.WriteString("### " + text + "\n\n")
			case "h4":
				text := strings.TrimSpace(s.Text())
				b.WriteString("#### " + text + "\n\n")
			case "h5":
				text := strings.TrimSpace(s.Text())
				b.WriteString("##### " + text + "\n\n")
			case "p":
				var pb strings.Builder
				convertInline(&pb, s)
				text := strings.TrimSpace(pb.String())
				if text != "" {
					b.WriteString(text + "\n\n")
				}
			case "ul":
				convertList(b, s, "ul", depth)
				b.WriteString("\n")
			case "ol":
				convertList(b, s, "ol", depth)
				b.WriteString("\n")
			case "dl":
				convertDL(b, s)
			case "table":
				convertTable(b, s)
				b.WriteString("\n")
			case "div":
				// Check if it's a table wrapper
				if s.HasClass("table") {
					convertNode(b, s, depth)
				} else if s.HasClass("level1") || s.HasClass("level2") || s.HasClass("level3") || s.HasClass("level4") || s.HasClass("level5") {
					convertNode(b, s, depth)
				} else {
					// Generic div, recurse
					convertNode(b, s, depth)
				}
			case "pre":
				text := s.Text()
				b.WriteString("```\n" + text + "\n```\n\n")
			case "hr":
				b.WriteString("---\n\n")
			case "br":
				b.WriteString("\n")
			case "blockquote":
				var qb strings.Builder
				convertNode(&qb, s, depth)
				for line := range strings.SplitSeq(strings.TrimSpace(qb.String()), "\n") {
					b.WriteString("> " + line + "\n")
				}
				b.WriteString("\n")
			default:
				// For any other element, just recurse
				convertNode(b, s, depth)
			}
		case html.TextNode:
			text := node.Data
			if strings.TrimSpace(text) != "" {
				b.WriteString(text)
			}
		}
	})
}

func convertInline(b *strings.Builder, sel *goquery.Selection) {
	sel.Contents().Each(func(i int, s *goquery.Selection) {
		node := s.Get(0)
		if node == nil {
			return
		}
		switch node.Type {
		case html.ElementNode:
			tag := goquery.NodeName(s)
			switch tag {
			case "strong", "b":
				var ib strings.Builder
				convertInline(&ib, s)
				text := ib.String()
				if text != "" {
					b.WriteString("**" + text + "**")
				}
			case "em", "i":
				var ib strings.Builder
				convertInline(&ib, s)
				text := ib.String()
				if text != "" {
					b.WriteString("*" + text + "*")
				}
			case "a":
				href, exists := s.Attr("href")
				var ib strings.Builder
				convertInline(&ib, s)
				text := ib.String()
				if exists && text != "" {
					// Convert internal DokuWiki links
					href = resolveLink(href)
					b.WriteString("[" + text + "](" + href + ")")
				} else if text != "" {
					b.WriteString(text)
				}
			case "code":
				b.WriteString("`" + s.Text() + "`")
			case "sup":
				b.WriteString("^" + s.Text())
			case "sub":
				b.WriteString("~" + s.Text())
			case "br":
				b.WriteString("\n")
			case "span":
				convertInline(b, s)
			case "del", "s":
				var ib strings.Builder
				convertInline(&ib, s)
				b.WriteString("~~" + ib.String() + "~~")
			case "u":
				convertInline(b, s)
			case "img":
				alt, _ := s.Attr("alt")
				src, _ := s.Attr("src")
				if src != "" {
					b.WriteString("![" + alt + "](" + src + ")")
				}
			default:
				convertInline(b, s)
			}
		case html.TextNode:
			b.WriteString(node.Data)
		}
	})
}

func convertList(b *strings.Builder, sel *goquery.Selection, listType string, depth int) {
	sel.Children().Each(func(i int, li *goquery.Selection) {
		if goquery.NodeName(li) != "li" {
			return
		}
		indent := strings.Repeat("  ", depth)
		prefix := "- "
		if listType == "ol" {
			prefix = fmt.Sprintf("%d. ", i+1)
		}

		// Get inline text (not from nested lists)
		var ib strings.Builder
		li.Contents().Each(func(j int, c *goquery.Selection) {
			node := c.Get(0)
			if node == nil {
				return
			}
			tag := goquery.NodeName(c)
			if tag == "ul" || tag == "ol" {
				return // Skip nested lists, handle below
			}
			if node.Type == html.ElementNode {
				convertInline(&ib, c)
			} else if node.Type == html.TextNode {
				ib.WriteString(node.Data)
			}
		})

		text := strings.TrimSpace(ib.String())
		if text != "" {
			b.WriteString(indent + prefix + text + "\n")
		}

		// Handle nested lists
		li.Children().Each(func(j int, c *goquery.Selection) {
			tag := goquery.NodeName(c)
			if tag == "ul" {
				convertList(b, c, "ul", depth+1)
			} else if tag == "ol" {
				convertList(b, c, "ol", depth+1)
			}
		})
	})
}

func convertDL(b *strings.Builder, sel *goquery.Selection) {
	sel.Children().Each(func(i int, s *goquery.Selection) {
		tag := goquery.NodeName(s)
		switch tag {
		case "dt":
			var ib strings.Builder
			convertInline(&ib, s)
			text := strings.TrimSpace(ib.String())
			b.WriteString("**" + text + "**\n")
		case "dd":
			var ib strings.Builder
			convertInline(&ib, s)
			text := strings.TrimSpace(ib.String())
			b.WriteString(": " + text + "\n\n")
		}
	})
}

func convertTable(b *strings.Builder, sel *goquery.Selection) {
	var rows [][]string
	var headerCount int

	// Process thead
	sel.Find("thead tr").Each(func(i int, tr *goquery.Selection) {
		var row []string
		tr.Find("th, td").Each(func(j int, td *goquery.Selection) {
			var ib strings.Builder
			convertInline(&ib, td)
			row = append(row, strings.TrimSpace(ib.String()))
		})
		rows = append(rows, row)
		headerCount++
	})

	// Process tbody rows (or direct tr children if no thead/tbody)
	bodyRows := sel.Find("tbody tr")
	if bodyRows.Length() == 0 {
		// No tbody, get trs that aren't in thead
		sel.Find("tr").Each(func(i int, tr *goquery.Selection) {
			if tr.ParentsFiltered("thead").Length() > 0 {
				return
			}
			var row []string
			tr.Find("th, td").Each(func(j int, td *goquery.Selection) {
				var ib strings.Builder
				convertInline(&ib, td)
				row = append(row, strings.TrimSpace(ib.String()))
			})
			if len(row) > 0 {
				rows = append(rows, row)
			}
		})
	} else {
		bodyRows.Each(func(i int, tr *goquery.Selection) {
			var row []string
			tr.Find("th, td").Each(func(j int, td *goquery.Selection) {
				var ib strings.Builder
				convertInline(&ib, td)
				row = append(row, strings.TrimSpace(ib.String()))
			})
			if len(row) > 0 {
				rows = append(rows, row)
			}
		})
	}

	if len(rows) == 0 {
		return
	}

	// Determine column widths
	numCols := 0
	for _, row := range rows {
		if len(row) > numCols {
			numCols = len(row)
		}
	}

	colWidths := make([]int, numCols)
	for _, row := range rows {
		for j, cell := range row {
			if len(cell) > colWidths[j] {
				colWidths[j] = len(cell)
			}
		}
	}

	// Write table
	for i, row := range rows {
		b.WriteString("|")
		for j := 0; j < numCols; j++ {
			cell := ""
			if j < len(row) {
				cell = row[j]
			}
			b.WriteString(" " + pad(cell, colWidths[j]) + " |")
		}
		b.WriteString("\n")

		// Write separator after header row
		if i == 0 && headerCount > 0 {
			b.WriteString("|")
			for j := 0; j < numCols; j++ {
				b.WriteString(" " + strings.Repeat("-", colWidths[j]) + " |")
			}
			b.WriteString("\n")
		} else if i == 0 && headerCount == 0 {
			// If there's no explicit header, still add a separator after first row
			b.WriteString("|")
			for j := 0; j < numCols; j++ {
				b.WriteString(" " + strings.Repeat("-", colWidths[j]) + " |")
			}
			b.WriteString("\n")
		}
	}
}

func pad(s string, width int) string {
	if len(s) >= width {
		return s
	}
	return s + strings.Repeat(" ", width-len(s))
}

func resolveLink(href string) string {
	// Convert DokuWiki internal links to relative markdown references
	if strings.Contains(href, "doku.php?id=") {
		parts := strings.SplitN(href, "id=", 2)
		if len(parts) == 2 {
			return parts[1]
		}
	}
	return href
}
