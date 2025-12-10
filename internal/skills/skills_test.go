package skills

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/stretchr/testify/require"
)

func TestLoadSkillsFromRepoAndCustomRoots(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(root, ".git"), 0o755))

	repoSkillDir := filepath.Join(root, ".crush", "skills", "repo")
	require.NoError(t, os.MkdirAll(repoSkillDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(repoSkillDir, "SKILL.md"), []byte(`---
name: repo-skill
description: from repo
---
`), 0o644))

	customRoot := filepath.Join(root, "custom-skills", "alpha")
	require.NoError(t, os.MkdirAll(customRoot, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(customRoot, "SKILL.md"), []byte(`---
name: custom
description: custom root
---
`), 0o644))

	cfg, err := config.Load(root, "", false)
	require.NoError(t, err)
	cfg.Options.SkillsDirs = []string{filepath.Dir(customRoot)}

	result := Load(cfg)
	require.Len(t, result.Skills, 2)
	require.Empty(t, result.Errors)

	names := []string{result.Skills[0].Name, result.Skills[1].Name}
	require.Contains(t, names, "repo-skill")
	require.Contains(t, names, "custom")

	summary := RenderSummary(result.Skills, result.Roots)
	require.True(t, strings.Contains(summary, "repo-skill"))
	require.True(t, strings.Contains(summary, "custom"))
}

func TestLoadReportsInvalidSkills(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(root, ".git"), 0o755))

	badDir := filepath.Join(root, ".crush", "skills", "bad")
	require.NoError(t, os.MkdirAll(badDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(badDir, "SKILL.md"), []byte("not-a-skill"), 0o644))

	cfg, err := config.Load(root, "", false)
	require.NoError(t, err)

	result := Load(cfg)
	require.Len(t, result.Skills, 0)
	require.Len(t, result.Errors, 1)
	require.True(t, strings.Contains(result.Errors[0].Err.Error(), "frontmatter"))
}
