// Command doccheck validates the handbook's documentation and agent bundle:
// relative Markdown links and anchors resolve, `.handbook/...` references
// point at real files, banned terms are absent, the root instruction file
// stays within budget, and rule, skill, and agent frontmatter is well formed.
//
// Usage: go run ./tools/doccheck [-root dir]
package main

import (
	"bufio"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
)

const rootInstructionBudget = 200

// bannedTerms are names from the project this handbook was derived from, and
// internal libraries that do not exist here. Matching is case-insensitive.
var bannedTerms = []string{
	"vektor",
	"domainkit",
	"commitplan",
	"grpcx",
	"tracex",
	"isvalidstruct",
	"buildandapply",
	"flow.group",
	"custom-gcl",
	"outbox-publisher",
	"setup-architecture",
	".dev/architecture",
}

var (
	linkPattern     = regexp.MustCompile(`\]\(([^)\s]+)\)`)
	handbookPattern = regexp.MustCompile(`\.handbook/([A-Za-z0-9_./-]+)`)
	headingPattern  = regexp.MustCompile(`^#{1,6}\s+(.*)$`)
)

type checker struct {
	root     string
	problems []string
	anchors  map[string][]string // markdown file → heading slugs
}

func main() {
	root := flag.String("root", ".", "handbook root directory")
	flag.Parse()

	c := &checker{root: *root, anchors: make(map[string][]string)}
	if err := c.run(); err != nil {
		fmt.Fprintln(os.Stderr, "doccheck:", err)
		os.Exit(2)
	}
	if len(c.problems) > 0 {
		slices.Sort(c.problems)
		for _, p := range c.problems {
			fmt.Println(p)
		}
		fmt.Printf("doccheck: %d problem(s)\n", len(c.problems))
		os.Exit(1)
	}
	fmt.Println("doccheck: ok")
}

func (c *checker) run() error {
	var markdown, text []string
	err := filepath.WalkDir(c.root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		name := d.Name()
		if d.IsDir() {
			if name == ".git" || name == ".idea" || name == ".vscode" {
				return filepath.SkipDir
			}
			return nil
		}
		switch filepath.Ext(name) {
		case ".md":
			markdown = append(markdown, path)
			text = append(text, path)
		case ".go", ".sh", ".yaml", ".yml", ".json", ".txt", ".proto", ".mod":
			text = append(text, path)
		default:
			if name == "Makefile" || name == "Dockerfile" || strings.HasPrefix(name, "Makefile.") {
				text = append(text, path)
			}
		}
		return nil
	})
	if err != nil {
		return err
	}

	for _, path := range text {
		if err := c.checkBannedTerms(path); err != nil {
			return err
		}
	}
	for _, path := range markdown {
		if err := c.checkMarkdown(path); err != nil {
			return err
		}
	}
	c.checkRootBudget()
	c.checkRules()
	c.checkSkills()
	c.checkAgents()
	return nil
}

func (c *checker) report(path string, line int, format string, args ...any) {
	rel, _ := filepath.Rel(c.root, path)
	c.problems = append(c.problems, fmt.Sprintf("%s:%d: %s", filepath.ToSlash(rel), line, fmt.Sprintf(format, args...)))
}

func (c *checker) checkBannedTerms(path string) error {
	if strings.HasSuffix(filepath.ToSlash(path), "tools/doccheck/main.go") {
		return nil // this file lists the terms
	}
	return eachLine(path, func(n int, line string) {
		lower := strings.ToLower(line)
		for _, term := range bannedTerms {
			if strings.Contains(lower, term) {
				c.report(path, n, "banned term %q", term)
			}
		}
	})
}

func (c *checker) checkMarkdown(path string) error {
	inFence := false
	return eachLine(path, func(n int, line string) {
		if strings.HasPrefix(strings.TrimSpace(line), "```") {
			inFence = !inFence
			return
		}
		if inFence {
			return
		}
		for _, m := range linkPattern.FindAllStringSubmatch(line, -1) {
			c.checkLink(path, n, m[1])
		}
		for _, m := range handbookPattern.FindAllStringSubmatch(stripInlineCode(line, false), -1) {
			c.checkHandbookRef(path, n, m[1])
		}
	})
}

func (c *checker) checkLink(from string, line int, target string) {
	if strings.Contains(target, "://") || strings.HasPrefix(target, "mailto:") {
		return
	}
	file, anchor, _ := strings.Cut(target, "#")
	resolved := from
	if file != "" {
		resolved = filepath.Join(filepath.Dir(from), filepath.FromSlash(file))
		if _, err := os.Stat(resolved); err != nil {
			c.report(from, line, "broken link %q", target)
			return
		}
	}
	if anchor != "" && strings.HasSuffix(resolved, ".md") {
		slugs, err := c.slugs(resolved)
		if err != nil {
			c.report(from, line, "read %q: %v", target, err)
			return
		}
		if !slices.Contains(slugs, anchor) {
			c.report(from, line, "broken anchor %q", target)
		}
	}
}

func (c *checker) checkHandbookRef(from string, line int, ref string) {
	ref = strings.TrimRight(ref, ".,;:")
	if ref == "" {
		return
	}
	if _, err := os.Stat(filepath.Join(c.root, filepath.FromSlash(ref))); err != nil {
		c.report(from, line, "reference .handbook/%s does not exist", ref)
	}
}

func (c *checker) slugs(path string) ([]string, error) {
	if s, ok := c.anchors[path]; ok {
		return s, nil
	}
	var slugs []string
	inFence := false
	err := eachLine(path, func(_ int, line string) {
		if strings.HasPrefix(strings.TrimSpace(line), "```") {
			inFence = !inFence
			return
		}
		if m := headingPattern.FindStringSubmatch(line); m != nil && !inFence {
			slugs = append(slugs, slugify(m[1]))
		}
	})
	c.anchors[path] = slugs
	return slugs, err
}

// slugify approximates GitHub's heading anchors: lowercase, drop punctuation
// except hyphens and underscores, spaces become hyphens.
func slugify(heading string) string {
	heading = strings.ToLower(stripInlineCode(heading, true))
	var b strings.Builder
	for _, r := range heading {
		switch {
		case r == ' ':
			b.WriteRune('-')
		case r == '-' || r == '_' || r >= 'a' && r <= 'z' || r >= '0' && r <= '9':
			b.WriteRune(r)
		case r > 127:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// stripInlineCode removes backticks; when keepContent is false it also drops
// placeholders such as <name> so they are not treated as paths.
func stripInlineCode(s string, keepContent bool) string {
	s = strings.ReplaceAll(s, "`", "")
	if keepContent {
		return s
	}
	return regexp.MustCompile(`<[^>]*>`).ReplaceAllString(s, " ")
}

func (c *checker) checkRootBudget() {
	path := filepath.Join(c.root, "guides", "ai-agent-instructions.md")
	n := 0
	if err := eachLine(path, func(int, string) { n++ }); err != nil {
		c.report(path, 0, "read root instructions: %v", err)
		return
	}
	if n > rootInstructionBudget {
		c.report(path, n, "root instruction file has %d lines; budget is %d", n, rootInstructionBudget)
	}
}

func (c *checker) checkRules() {
	files, _ := filepath.Glob(filepath.Join(c.root, ".agents", "rules", "*.md"))
	if len(files) == 0 {
		c.report(filepath.Join(c.root, ".agents", "rules"), 0, "no rule files found")
	}
	for _, path := range files {
		fm := frontmatter(path)
		if !strings.Contains(fm, "\npaths:\n") || !strings.Contains(fm, "\n  - ") {
			c.report(path, 1, "rule frontmatter must declare a paths: list")
		}
		if strings.Contains(fm, "\nglobs:") {
			c.report(path, 1, "use paths:, not globs:")
		}
	}
}

func (c *checker) checkSkills() {
	dirs, _ := filepath.Glob(filepath.Join(c.root, ".agents", "skills", "*"))
	for _, dir := range dirs {
		path := filepath.Join(dir, "SKILL.md")
		fm := frontmatter(path)
		if fm == "" {
			c.report(path, 1, "missing SKILL.md or frontmatter")
			continue
		}
		if !strings.Contains(fm, "\nname: "+filepath.Base(dir)+"\n") {
			c.report(path, 1, "frontmatter name must equal directory name %q", filepath.Base(dir))
		}
		if !strings.Contains(fm, "\ndescription:") {
			c.report(path, 1, "frontmatter needs a description")
		}
	}
}

func (c *checker) checkAgents() {
	files, _ := filepath.Glob(filepath.Join(c.root, ".agents", "agents", "*.md"))
	for _, path := range files {
		fm := frontmatter(path)
		name := strings.TrimSuffix(filepath.Base(path), ".md")
		if !strings.Contains(fm, "\nname: "+name+"\n") || !strings.Contains(fm, "\ndescription:") {
			c.report(path, 1, "agent frontmatter needs name %q and a description", name)
		}
		if strings.Contains(fm, "\nmodel:") {
			c.report(path, 1, "agents must not pin a model")
		}
	}
}

// frontmatter returns the YAML block between the leading --- lines, with a
// leading newline so keys can be matched as "\nkey:".
func frontmatter(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	text := strings.ReplaceAll(string(data), "\r\n", "\n")
	if !strings.HasPrefix(text, "---\n") {
		return ""
	}
	end := strings.Index(text[4:], "\n---")
	if end < 0 {
		return ""
	}
	return "\n" + text[4:4+end] + "\n"
}

func eachLine(path string, fn func(n int, line string)) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 1<<20)
	n := 0
	for sc.Scan() {
		n++
		fn(n, sc.Text())
	}
	return sc.Err()
}
