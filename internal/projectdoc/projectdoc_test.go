package projectdoc

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/stretchr/testify/require"
)

func TestLoadOrdersDocsFromRootToCWD(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(root, ".git"), 0o755))

	require.NoError(t, os.WriteFile(filepath.Join(root, "AGENTS.md"), []byte("root-doc"), 0o644))

	nested := filepath.Join(root, "nested")
	require.NoError(t, os.MkdirAll(nested, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(nested, "AGENTS.override.md"), []byte("child-doc"), 0o644))

	cfg, err := config.Load(nested, "", false)
	require.NoError(t, err)

	docs, warnings := Load(cfg)
	require.Empty(t, warnings)
	require.Len(t, docs, 2)
	require.True(t, strings.Contains(docs[0].Content, "root-doc"))
	require.True(t, strings.Contains(docs[1].Content, "child-doc"))
}

func TestLoadRespectsFallbackAndBudget(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(root, ".git"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, "TEAM.md"), []byte("abcdefghi"), 0o644))

	cfg, err := config.Load(root, "", false)
	require.NoError(t, err)
	cfg.Options.ProjectDocFallbackFilenames = []string{"TEAM.md"}
	cfg.Options.ProjectDocMaxBytes = 5

	docs, warnings := Load(cfg)
	require.Len(t, docs, 1)
	require.Equal(t, "abcde", docs[0].Content)
	require.Len(t, warnings, 1)
}
