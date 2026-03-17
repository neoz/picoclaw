package skills

import (
	"os"
	"path/filepath"
	"testing"
)

// helper to create a skill directory with a SKILL.md file
func createSkill(t *testing.T, baseDir, name, content string) {
	t.Helper()
	dir := filepath.Join(baseDir, name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

const yamlSkillContent = `---
name: test-skill
description: A test skill
---
This is the skill body.
`

const jsonSkillContent = `---
{"name": "json-skill", "description": "A JSON metadata skill"}
---
JSON skill body.
`

const noFrontmatterContent = `Just plain content with no frontmatter.
`

func TestNewSkillsLoader(t *testing.T) {
	sl := NewSkillsLoader("/workspace", "/global", "/builtin")
	if sl.workspace != "/workspace" {
		t.Errorf("expected workspace /workspace, got %s", sl.workspace)
	}
	if sl.workspaceSkills != filepath.Join("/workspace", "skills") {
		t.Errorf("unexpected workspaceSkills: %s", sl.workspaceSkills)
	}
	if sl.globalSkills != "/global" {
		t.Errorf("expected globalSkills /global, got %s", sl.globalSkills)
	}
	if sl.builtinSkills != "/builtin" {
		t.Errorf("expected builtinSkills /builtin, got %s", sl.builtinSkills)
	}
}

func TestListSkills_AllSources(t *testing.T) {
	tmp := t.TempDir()
	wsSkills := filepath.Join(tmp, "workspace", "skills")
	globalSkills := filepath.Join(tmp, "global")
	builtinSkills := filepath.Join(tmp, "builtin")

	createSkill(t, wsSkills, "alpha", yamlSkillContent)
	createSkill(t, globalSkills, "beta", jsonSkillContent)
	createSkill(t, builtinSkills, "gamma", noFrontmatterContent)

	sl := &SkillsLoader{
		workspace:       filepath.Join(tmp, "workspace"),
		workspaceSkills: wsSkills,
		globalSkills:    globalSkills,
		builtinSkills:   builtinSkills,
	}

	skills := sl.ListSkills()
	if len(skills) != 3 {
		t.Fatalf("expected 3 skills, got %d", len(skills))
	}

	byName := make(map[string]SkillInfo)
	for _, s := range skills {
		byName[s.Name] = s
	}

	// yamlSkillContent has name: test-skill, overriding dir name "alpha"
	if s, ok := byName["test-skill"]; !ok {
		t.Error("missing test-skill skill (from alpha dir)")
	} else {
		if s.Source != "workspace" {
			t.Errorf("test-skill source: expected workspace, got %s", s.Source)
		}
		if s.Description != "A test skill" {
			t.Errorf("test-skill description: got %q", s.Description)
		}
	}

	// jsonSkillContent has name: json-skill, overriding dir name "beta"
	if s, ok := byName["json-skill"]; !ok {
		t.Error("missing json-skill skill (from beta dir)")
	} else {
		if s.Source != "global" {
			t.Errorf("json-skill source: expected global, got %s", s.Source)
		}
		if s.Description != "A JSON metadata skill" {
			t.Errorf("json-skill description: got %q", s.Description)
		}
	}

	if s, ok := byName["gamma"]; !ok {
		t.Error("missing gamma skill")
	} else {
		if s.Source != "builtin" {
			t.Errorf("gamma source: expected builtin, got %s", s.Source)
		}
	}
}

func TestListSkills_WorkspaceOverridesGlobal(t *testing.T) {
	tmp := t.TempDir()
	wsSkills := filepath.Join(tmp, "workspace", "skills")
	globalSkills := filepath.Join(tmp, "global")

	createSkill(t, wsSkills, "shared", "---\nname: shared\ndescription: workspace version\n---\nworkspace body")
	createSkill(t, globalSkills, "shared", "---\nname: shared\ndescription: global version\n---\nglobal body")

	sl := &SkillsLoader{
		workspace:       filepath.Join(tmp, "workspace"),
		workspaceSkills: wsSkills,
		globalSkills:    globalSkills,
	}

	skills := sl.ListSkills()
	if len(skills) != 1 {
		t.Fatalf("expected 1 skill, got %d", len(skills))
	}
	if skills[0].Source != "workspace" {
		t.Errorf("expected workspace source, got %s", skills[0].Source)
	}
	if skills[0].Description != "workspace version" {
		t.Errorf("expected workspace description, got %q", skills[0].Description)
	}
}

func TestListSkills_GlobalOverridesBuiltin(t *testing.T) {
	tmp := t.TempDir()
	globalSkills := filepath.Join(tmp, "global")
	builtinSkills := filepath.Join(tmp, "builtin")

	createSkill(t, globalSkills, "shared", "---\nname: shared\ndescription: global version\n---\nglobal body")
	createSkill(t, builtinSkills, "shared", "---\nname: shared\ndescription: builtin version\n---\nbuiltin body")

	sl := &SkillsLoader{
		workspace:       filepath.Join(tmp, "workspace"),
		workspaceSkills: filepath.Join(tmp, "workspace", "skills"),
		globalSkills:    globalSkills,
		builtinSkills:   builtinSkills,
	}

	skills := sl.ListSkills()
	if len(skills) != 1 {
		t.Fatalf("expected 1 skill, got %d", len(skills))
	}
	if skills[0].Source != "global" {
		t.Errorf("expected global source, got %s", skills[0].Source)
	}
}

func TestListSkills_WorkspaceOverridesBuiltin(t *testing.T) {
	tmp := t.TempDir()
	wsSkills := filepath.Join(tmp, "workspace", "skills")
	builtinSkills := filepath.Join(tmp, "builtin")

	createSkill(t, wsSkills, "shared", "---\nname: shared\ndescription: ws version\n---\nws body")
	createSkill(t, builtinSkills, "shared", "---\nname: shared\ndescription: builtin version\n---\nbuiltin body")

	sl := &SkillsLoader{
		workspace:       filepath.Join(tmp, "workspace"),
		workspaceSkills: wsSkills,
		builtinSkills:   builtinSkills,
	}

	skills := sl.ListSkills()
	if len(skills) != 1 {
		t.Fatalf("expected 1 skill, got %d", len(skills))
	}
	if skills[0].Source != "workspace" {
		t.Errorf("expected workspace source, got %s", skills[0].Source)
	}
}

func TestListSkills_EmptyDirs(t *testing.T) {
	sl := &SkillsLoader{}
	skills := sl.ListSkills()
	if len(skills) != 0 {
		t.Errorf("expected 0 skills, got %d", len(skills))
	}
}

func TestListSkills_NonExistentDirs(t *testing.T) {
	sl := &SkillsLoader{
		workspaceSkills: "/nonexistent/workspace/skills",
		globalSkills:    "/nonexistent/global",
		builtinSkills:   "/nonexistent/builtin",
	}
	skills := sl.ListSkills()
	if len(skills) != 0 {
		t.Errorf("expected 0 skills, got %d", len(skills))
	}
}

func TestListSkills_SkipsNonDirs(t *testing.T) {
	tmp := t.TempDir()
	wsSkills := filepath.Join(tmp, "workspace", "skills")
	os.MkdirAll(wsSkills, 0o755)
	// Create a file (not a directory) in the skills dir
	os.WriteFile(filepath.Join(wsSkills, "not-a-dir.txt"), []byte("hello"), 0o644)

	sl := &SkillsLoader{
		workspace:       filepath.Join(tmp, "workspace"),
		workspaceSkills: wsSkills,
	}

	skills := sl.ListSkills()
	if len(skills) != 0 {
		t.Errorf("expected 0 skills, got %d", len(skills))
	}
}

func TestListSkills_SkipsDirWithoutSkillMd(t *testing.T) {
	tmp := t.TempDir()
	wsSkills := filepath.Join(tmp, "workspace", "skills")
	// Create a directory but no SKILL.md inside
	os.MkdirAll(filepath.Join(wsSkills, "empty-skill"), 0o755)

	sl := &SkillsLoader{
		workspace:       filepath.Join(tmp, "workspace"),
		workspaceSkills: wsSkills,
	}

	skills := sl.ListSkills()
	if len(skills) != 0 {
		t.Errorf("expected 0 skills, got %d", len(skills))
	}
}

func TestLoadSkill_PriorityOrder(t *testing.T) {
	tmp := t.TempDir()
	wsSkills := filepath.Join(tmp, "workspace", "skills")
	globalSkills := filepath.Join(tmp, "global")
	builtinSkills := filepath.Join(tmp, "builtin")

	createSkill(t, wsSkills, "myskill", "---\nname: myskill\n---\nworkspace content")
	createSkill(t, globalSkills, "myskill", "---\nname: myskill\n---\nglobal content")
	createSkill(t, builtinSkills, "myskill", "---\nname: myskill\n---\nbuiltin content")

	sl := &SkillsLoader{
		workspace:       filepath.Join(tmp, "workspace"),
		workspaceSkills: wsSkills,
		globalSkills:    globalSkills,
		builtinSkills:   builtinSkills,
	}

	// workspace wins
	content, ok := sl.LoadSkill("myskill")
	if !ok {
		t.Fatal("expected skill to be found")
	}
	if content != "workspace content" {
		t.Errorf("expected workspace content, got %q", content)
	}

	// remove workspace, global wins
	sl.workspaceSkills = ""
	content, ok = sl.LoadSkill("myskill")
	if !ok {
		t.Fatal("expected skill to be found")
	}
	if content != "global content" {
		t.Errorf("expected global content, got %q", content)
	}

	// remove global, builtin wins
	sl.globalSkills = ""
	content, ok = sl.LoadSkill("myskill")
	if !ok {
		t.Fatal("expected skill to be found")
	}
	if content != "builtin content" {
		t.Errorf("expected builtin content, got %q", content)
	}
}

func TestLoadSkill_NotFound(t *testing.T) {
	tmp := t.TempDir()
	sl := &SkillsLoader{
		workspaceSkills: filepath.Join(tmp, "ws"),
		globalSkills:    filepath.Join(tmp, "global"),
		builtinSkills:   filepath.Join(tmp, "builtin"),
	}

	content, ok := sl.LoadSkill("nonexistent")
	if ok {
		t.Error("expected skill not to be found")
	}
	if content != "" {
		t.Errorf("expected empty content, got %q", content)
	}
}

func TestLoadSkill_StripsFrontmatter(t *testing.T) {
	tmp := t.TempDir()
	wsSkills := filepath.Join(tmp, "workspace", "skills")
	// Use single-line frontmatter that stripFrontmatter regex can handle
	createSkill(t, wsSkills, "test", "---\nname: test\n---\nThis is the skill body.\n")

	sl := &SkillsLoader{
		workspace:       filepath.Join(tmp, "workspace"),
		workspaceSkills: wsSkills,
	}

	content, ok := sl.LoadSkill("test")
	if !ok {
		t.Fatal("expected skill to be found")
	}
	if content != "This is the skill body.\n" {
		t.Errorf("unexpected content: %q", content)
	}
}

func TestLoadSkillsForContext_Empty(t *testing.T) {
	sl := &SkillsLoader{}
	result := sl.LoadSkillsForContext(nil)
	if result != "" {
		t.Errorf("expected empty result, got %q", result)
	}
	result = sl.LoadSkillsForContext([]string{})
	if result != "" {
		t.Errorf("expected empty result, got %q", result)
	}
}

func TestLoadSkillsForContext_Multiple(t *testing.T) {
	tmp := t.TempDir()
	wsSkills := filepath.Join(tmp, "workspace", "skills")
	createSkill(t, wsSkills, "alpha", "---\nname: alpha\n---\nalpha body")
	createSkill(t, wsSkills, "beta", "---\nname: beta\n---\nbeta body")

	sl := &SkillsLoader{
		workspace:       filepath.Join(tmp, "workspace"),
		workspaceSkills: wsSkills,
	}

	result := sl.LoadSkillsForContext([]string{"alpha", "beta"})
	if result == "" {
		t.Fatal("expected non-empty result")
	}
	if !contains(result, "### Skill: alpha") {
		t.Error("missing alpha header")
	}
	if !contains(result, "### Skill: beta") {
		t.Error("missing beta header")
	}
	if !contains(result, "alpha body") {
		t.Error("missing alpha body")
	}
	if !contains(result, "beta body") {
		t.Error("missing beta body")
	}
	if !contains(result, "---") {
		t.Error("missing separator")
	}
}

func TestLoadSkillsForContext_SkipsMissing(t *testing.T) {
	tmp := t.TempDir()
	wsSkills := filepath.Join(tmp, "workspace", "skills")
	createSkill(t, wsSkills, "exists", "---\nname: exists\n---\nexists body")

	sl := &SkillsLoader{
		workspace:       filepath.Join(tmp, "workspace"),
		workspaceSkills: wsSkills,
	}

	result := sl.LoadSkillsForContext([]string{"exists", "missing"})
	if !contains(result, "exists body") {
		t.Error("missing exists body")
	}
	if contains(result, "missing") {
		t.Error("should not contain missing skill")
	}
}

func TestBuildSkillsSummary_Empty(t *testing.T) {
	sl := &SkillsLoader{}
	result := sl.BuildSkillsSummary()
	if result != "" {
		t.Errorf("expected empty result, got %q", result)
	}
}

func TestBuildSkillsSummary_XMLOutput(t *testing.T) {
	tmp := t.TempDir()
	wsSkills := filepath.Join(tmp, "workspace", "skills")
	createSkill(t, wsSkills, "test", yamlSkillContent)

	sl := &SkillsLoader{
		workspace:       filepath.Join(tmp, "workspace"),
		workspaceSkills: wsSkills,
	}

	result := sl.BuildSkillsSummary()
	if !contains(result, "<skills>") {
		t.Error("missing <skills> tag")
	}
	if !contains(result, "</skills>") {
		t.Error("missing </skills> tag")
	}
	// yamlSkillContent has name: test-skill, overriding dir name "test"
	if !contains(result, "<name>test-skill</name>") {
		t.Error("missing skill name")
	}
	if !contains(result, "<description>A test skill</description>") {
		t.Error("missing skill description")
	}
	if !contains(result, "<source>workspace</source>") {
		t.Error("missing source")
	}
}

func TestBuildSkillsSummary_EscapesXML(t *testing.T) {
	tmp := t.TempDir()
	wsSkills := filepath.Join(tmp, "workspace", "skills")
	createSkill(t, wsSkills, "special", "---\nname: special\ndescription: Uses <tags> & \"quotes\"\n---\nbody")

	sl := &SkillsLoader{
		workspace:       filepath.Join(tmp, "workspace"),
		workspaceSkills: wsSkills,
	}

	result := sl.BuildSkillsSummary()
	if !contains(result, "&lt;tags&gt;") {
		t.Error("expected escaped angle brackets")
	}
	if !contains(result, "&amp;") {
		t.Error("expected escaped ampersand")
	}
}

func TestGetSkillMetadata_YAML(t *testing.T) {
	tmp := t.TempDir()
	skillFile := filepath.Join(tmp, "SKILL.md")
	os.WriteFile(skillFile, []byte(yamlSkillContent), 0o644)

	sl := &SkillsLoader{}
	meta := sl.getSkillMetadata(skillFile)
	if meta == nil {
		t.Fatal("expected metadata")
	}
	if meta.Name != "test-skill" {
		t.Errorf("expected name test-skill, got %q", meta.Name)
	}
	if meta.Description != "A test skill" {
		t.Errorf("expected description 'A test skill', got %q", meta.Description)
	}
}

func TestGetSkillMetadata_JSON(t *testing.T) {
	tmp := t.TempDir()
	skillFile := filepath.Join(tmp, "SKILL.md")
	os.WriteFile(skillFile, []byte(jsonSkillContent), 0o644)

	sl := &SkillsLoader{}
	meta := sl.getSkillMetadata(skillFile)
	if meta == nil {
		t.Fatal("expected metadata")
	}
	if meta.Name != "json-skill" {
		t.Errorf("expected name json-skill, got %q", meta.Name)
	}
	if meta.Description != "A JSON metadata skill" {
		t.Errorf("expected description 'A JSON metadata skill', got %q", meta.Description)
	}
}

func TestGetSkillMetadata_NoFrontmatter(t *testing.T) {
	tmp := t.TempDir()
	dir := filepath.Join(tmp, "myskill")
	os.MkdirAll(dir, 0o755)
	skillFile := filepath.Join(dir, "SKILL.md")
	os.WriteFile(skillFile, []byte(noFrontmatterContent), 0o644)

	sl := &SkillsLoader{}
	meta := sl.getSkillMetadata(skillFile)
	if meta == nil {
		t.Fatal("expected metadata")
	}
	// Falls back to directory name
	if meta.Name != "myskill" {
		t.Errorf("expected name myskill, got %q", meta.Name)
	}
	if meta.Description != "Just plain content with no frontmatter." {
		t.Errorf("expected body paragraph as description, got %q", meta.Description)
	}
}

func TestGetSkillMetadata_NonexistentFile(t *testing.T) {
	sl := &SkillsLoader{}
	meta := sl.getSkillMetadata("/nonexistent/SKILL.md")
	if meta != nil {
		t.Error("expected nil metadata for nonexistent file")
	}
}

func TestParseSimpleYAML(t *testing.T) {
	sl := &SkillsLoader{}

	tests := []struct {
		name     string
		input    string
		expected map[string]string
	}{
		{
			name:  "basic key-value",
			input: "name: test\ndescription: a description",
			expected: map[string]string{
				"name":        "test",
				"description": "a description",
			},
		},
		{
			name:  "quoted values",
			input: "name: \"quoted\"\ndescription: 'single quoted'",
			expected: map[string]string{
				"name":        "quoted",
				"description": "single quoted",
			},
		},
		{
			name:     "skips comments and blanks",
			input:    "# comment\n\nname: test",
			expected: map[string]string{"name": "test"},
		},
		{
			name:     "value with colon",
			input:    "description: http://example.com:8080/path",
			expected: map[string]string{"description": "http://example.com:8080/path"},
		},
		{
			name:     "empty input",
			input:    "",
			expected: map[string]string{},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := sl.parseSimpleYAML(tc.input)
			for k, v := range tc.expected {
				if result[k] != v {
					t.Errorf("key %q: expected %q, got %q", k, v, result[k])
				}
			}
			if len(result) != len(tc.expected) {
				t.Errorf("expected %d entries, got %d", len(tc.expected), len(result))
			}
		})
	}
}

func TestExtractFrontmatter(t *testing.T) {
	sl := &SkillsLoader{}

	t.Run("yaml frontmatter", func(t *testing.T) {
		input := "---\nname: test\ndescription: desc\n---\nbody"
		result := sl.extractFrontmatter(input)
		if result != "name: test\ndescription: desc" {
			t.Errorf("unexpected result: %q", result)
		}
	})

	t.Run("no frontmatter", func(t *testing.T) {
		result := sl.extractFrontmatter("just content")
		if result != "" {
			t.Errorf("expected empty, got %q", result)
		}
	})
}

func TestSplitFrontmatter(t *testing.T) {
	tests := []struct {
		name              string
		input             string
		expectedFrontmatter string
		expectedBody      string
	}{
		{
			name:              "single-line frontmatter",
			input:             "---\nname: test\n---\nbody content",
			expectedFrontmatter: "name: test",
			expectedBody:      "body content",
		},
		{
			name:              "multi-line YAML frontmatter",
			input:             "---\nname: test-skill\ndescription: A test skill\ntags:\n  - foo\n  - bar\n---\nThis is the body.",
			expectedFrontmatter: "name: test-skill\ndescription: A test skill\ntags:\n  - foo\n  - bar",
			expectedBody:      "This is the body.",
		},
		{
			name:              "no frontmatter",
			input:             "just body content",
			expectedFrontmatter: "",
			expectedBody:      "just body content",
		},
		{
			name:              "no closing delimiter",
			input:             "---\nname: test\nbody without close",
			expectedFrontmatter: "",
			expectedBody:      "---\nname: test\nbody without close",
		},
		{
			name:              "empty frontmatter",
			input:             "---\n---\nbody after empty frontmatter",
			expectedFrontmatter: "",
			expectedBody:      "body after empty frontmatter",
		},
		{
			name:              "frontmatter with blank lines inside",
			input:             "---\nname: test\n\ndescription: has blank line\n---\nbody",
			expectedFrontmatter: "name: test\n\ndescription: has blank line",
			expectedBody:      "body",
		},
		{
			name:              "body with leading newlines stripped",
			input:             "---\nname: test\n---\n\n\nbody after newlines",
			expectedFrontmatter: "name: test",
			expectedBody:      "body after newlines",
		},
		{
			name:              "CRLF line endings",
			input:             "---\r\nname: test\r\ndescription: crlf\r\n---\r\nbody with crlf",
			expectedFrontmatter: "name: test\ndescription: crlf",
			expectedBody:      "body with crlf",
		},
		{
			name:              "does not start with delimiter",
			input:             "some text\n---\nname: test\n---\nbody",
			expectedFrontmatter: "",
			expectedBody:      "some text\n---\nname: test\n---\nbody",
		},
		{
			name:              "empty input",
			input:             "",
			expectedFrontmatter: "",
			expectedBody:      "",
		},
		{
			name:              "only delimiters",
			input:             "---\n---",
			expectedFrontmatter: "",
			expectedBody:      "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			fm, body := splitFrontmatter(tc.input)
			if fm != tc.expectedFrontmatter {
				t.Errorf("frontmatter: expected %q, got %q", tc.expectedFrontmatter, fm)
			}
			if body != tc.expectedBody {
				t.Errorf("body: expected %q, got %q", tc.expectedBody, body)
			}
		})
	}
}

func TestStripFrontmatter(t *testing.T) {
	sl := &SkillsLoader{}

	t.Run("with frontmatter", func(t *testing.T) {
		input := "---\nname: test\n---\nbody content"
		result := sl.stripFrontmatter(input)
		if result != "body content" {
			t.Errorf("unexpected result: %q", result)
		}
	})

	t.Run("without frontmatter", func(t *testing.T) {
		input := "just body content"
		result := sl.stripFrontmatter(input)
		if result != "just body content" {
			t.Errorf("unexpected result: %q", result)
		}
	})

	t.Run("multi-line YAML frontmatter", func(t *testing.T) {
		input := "---\nname: test-skill\ndescription: A test skill\ntags:\n  - foo\n  - bar\n---\nThis is the skill body.\n"
		result := sl.stripFrontmatter(input)
		if result != "This is the skill body.\n" {
			t.Errorf("unexpected result: %q", result)
		}
	})
}

func TestEscapeXML(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"hello", "hello"},
		{"<tag>", "&lt;tag&gt;"},
		{"a & b", "a &amp; b"},
		{"<a>&b</a>", "&lt;a&gt;&amp;b&lt;/a&gt;"},
		{"", ""},
	}

	for _, tc := range tests {
		result := escapeXML(tc.input)
		if result != tc.expected {
			t.Errorf("escapeXML(%q): expected %q, got %q", tc.input, tc.expected, result)
		}
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsStr(s, substr))
}

func containsStr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
