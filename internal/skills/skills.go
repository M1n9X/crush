package skills

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/fsext"
	"github.com/charmbracelet/crush/internal/home"
	"gopkg.in/yaml.v3"
)

const (
	skillFileName = "SKILL.md"
	maxNameLength = 100
	maxDescLength = 500
	skillsDirName = "skills"
	configDirName = ".crush"
)

// Skill represents validated metadata loaded from a SKILL.md file.
type Skill struct {
	Name        string
	Description string
	Path        string
}

// Error captures a problem encountered while loading a skill file.
type Error struct {
	Path string
	Err  error
}

// Result bundles discovered skills and any errors.
type Result struct {
	Skills []Skill
	Errors []Error
	Roots  []string
}

// Load discovers and validates SKILL.md files under configured roots. Only
// frontmatter name/description are injected; bodies stay on disk.
func Load(cfg *config.Config) Result {
	var result Result

	roots := skillRoots(cfg)
	seenRoots := map[string]struct{}{}
	for _, root := range roots {
		root = filepath.Clean(root)
		if root == "" {
			continue
		}
		if _, ok := seenRoots[root]; ok {
			continue
		}
		seenRoots[root] = struct{}{}
		result.Roots = append(result.Roots, root)
		walkSkills(root, &result)
	}

	slices.SortFunc(result.Skills, func(a, b Skill) int {
		return cmpStrings(a.Name, b.Name, a.Path, b.Path)
	})

	return result
}

// RenderSummary returns a compact textual summary of discovered skills. If no
// skills are present, an empty string is returned.
func RenderSummary(skills []Skill, roots []string) string {
	if len(skills) == 0 {
		return ""
	}

	var lines []string
	lines = append(lines, "## Skills")
	if len(roots) > 0 {
		shortRoots := make([]string, 0, len(roots))
		for _, root := range roots {
			shortRoots = append(shortRoots, filepath.ToSlash(home.Short(root)))
		}
		lines = append(lines, fmt.Sprintf("Discovered at startup under: %s", strings.Join(shortRoots, ", ")))
	}
	lines = append(lines, "Each entry lists the name, description, and file path. Open the source when you decide to use a skill; bodies are kept on disk.")

	for _, skill := range skills {
		lines = append(lines, fmt.Sprintf("- %s: %s (file: %s)", skill.Name, skill.Description, filepath.ToSlash(skill.Path)))
	}

	lines = append(lines,
		"- When a task or user mention matches a skill, open its SKILL.md and follow it.",
		"- Load only what you need: open referenced files/templates selectively instead of bulk-loading.",
		"- If a skill path is missing or unreadable, say so briefly and continue with the best fallback.",
	)

	return strings.Join(lines, "\n")
}

func skillRoots(cfg *config.Config) []string {
	var roots []string

	if len(cfg.Options.SkillsDirs) > 0 {
		for _, dir := range cfg.Options.SkillsDirs {
			if dir == "" {
				continue
			}
			roots = append(roots, home.Long(dir))
		}
	} else {
		roots = append(roots, filepath.Join(home.Dir(), configDirName, skillsDirName))
	}

	if repoRoot := fsext.FindGitRoot(cfg.WorkingDir()); repoRoot != "" {
		roots = append(roots, filepath.Join(repoRoot, configDirName, skillsDirName))
	} else {
		roots = append(roots, filepath.Join(cfg.WorkingDir(), configDirName, skillsDirName))
	}

	return roots
}

func walkSkills(root string, result *Result) {
	info, err := os.Stat(root)
	if err != nil || !info.IsDir() {
		return
	}

	filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}

		name := d.Name()
		if strings.HasPrefix(name, ".") {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		if d.Type()&os.ModeSymlink != 0 {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		if d.IsDir() {
			return nil
		}

		if name != skillFileName {
			return nil
		}

		skill, parseErr := parseSkill(path)
		if parseErr != nil {
			result.Errors = append(result.Errors, Error{Path: path, Err: parseErr})
			return nil
		}

		result.Skills = append(result.Skills, *skill)
		return nil
	})
}

func parseSkill(path string) (*Skill, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read file: %w", err)
	}

	frontmatter, ok := extractFrontmatter(string(data))
	if !ok {
		return nil, fmt.Errorf("missing YAML frontmatter delimited by ---")
	}

	var meta struct {
		Name        string `yaml:"name"`
		Description string `yaml:"description"`
	}
	if err := yaml.Unmarshal([]byte(frontmatter), &meta); err != nil {
		return nil, fmt.Errorf("invalid YAML: %w", err)
	}

	name := sanitize(meta.Name)
	desc := sanitize(meta.Description)

	if err := validateField(name, maxNameLength, "name"); err != nil {
		return nil, err
	}
	if err := validateField(desc, maxDescLength, "description"); err != nil {
		return nil, err
	}

	abs, err := filepath.Abs(path)
	if err != nil {
		abs = path
	}

	return &Skill{
		Name:        name,
		Description: desc,
		Path:        abs,
	}, nil
}

func extractFrontmatter(content string) (string, bool) {
	lines := strings.Split(content, "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return "", false
	}
	var fm []string
	foundClosing := false
	for _, line := range lines[1:] {
		if strings.TrimSpace(line) == "---" {
			foundClosing = true
			break
		}
		fm = append(fm, line)
	}
	if len(fm) == 0 || !foundClosing {
		return "", false
	}
	return strings.Join(fm, "\n"), true
}

func sanitize(value string) string {
	return strings.Join(strings.Fields(value), " ")
}

func validateField(value string, maxLen int, field string) error {
	if value == "" {
		return fmt.Errorf("missing field %s", field)
	}
	if len(value) > maxLen {
		return fmt.Errorf("invalid %s: exceeds %d characters", field, maxLen)
	}
	return nil
}

func cmpStrings(a, b, aPath, bPath string) int {
	if a == b {
		return strings.Compare(aPath, bPath)
	}
	return strings.Compare(a, b)
}
