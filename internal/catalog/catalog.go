package catalog

import (
	"encoding/json"
	"os"
	"strings"
)

type Category struct {
	ID            string `json:"id"`
	Label         string `json:"label"`
	Required      bool   `json:"required"`
	Priority      int    `json:"priority"`
	ApplyMethod   string `json:"apply_method"`
	TargetFileEnv string `json:"target_file_env"`
	TargetFile    string `json:"target_file"`
	CommentPrefix string `json:"comment_prefix"`
}

// ResolveTargetFile picks the category's target file per the README: the
// env var named by TargetFileEnv if it's set, otherwise TargetFile, with a
// leading "~" expanded to home.
func ResolveTargetFile(c Category, home string) string {
	path := c.TargetFile
	if c.TargetFileEnv != "" {
		if v := os.Getenv(c.TargetFileEnv); v != "" {
			path = v
		}
	}
	if path == "~" {
		return home
	}
	if strings.HasPrefix(path, "~/") {
		return home + path[1:]
	}
	return path
}

// Item's stable identity is FilePath: the data repo's index.json has no
// separate "id" field, and file_path is already unique per item (and is
// what depends_on entries and item fetches key off of).
type Item struct {
	Name          string   `json:"name"`
	Description   string   `json:"description"`
	CategoryID    string   `json:"category_id"`
	FilePath      string   `json:"file_path"`
	Previewcolors []string `json:"preview_colors"`
	Provides      struct {
		Aliases   []string `json:"aliases"`
		Functions []string `json:"functions"`
	} `json:"provides"`
	Version   string   `json:"version"`
	Hash      string   `json:"hash"`
	DependsOn []string `json:"depends_on"`
}

type Index struct {
	Categories []Category `json:"categories"`
	Items      []Item     `json:"items"`
}

func ParseIndex(data []byte) (*Index, error) {
	var idx Index
	if err := json.Unmarshal(data, &idx); err != nil {
		return nil, err
	}
	return &idx, nil
}

// ExtractBody strips the YAML frontmatter (delimited by the first two "---"
// lines) and returns everything after it. It splits at most twice so a
// "---" appearing literally inside the body (real example: oh-my-zsh's
// "sunrise" theme sets PROMPTPREFIX="---") doesn't truncate the content.
func ExtractBody(data []byte) string {
	parts := strings.SplitN(string(data), "---", 3)
	if len(parts) < 3 {
		return ""
	}
	return strings.TrimSpace(parts[2])
}
