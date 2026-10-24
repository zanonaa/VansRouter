package handlers

import (
	"fmt"
	"strings"
	"testing"
)

func TestTokenSaverRTKAutoDetectAndShrink(t *testing.T) {
	gitDiffLines := []string{"diff --git a/main.go b/main.go", "index 111..222 100644", "--- a/main.go", "+++ b/main.go", "@@ -1,2 +1,2 @@"}
	for index := 0; index < 120; index++ {
		gitDiffLines = append(gitDiffLines, fmt.Sprintf("+line %d with enough content to keep the hunk realistic", index))
	}
	gitStatusLines := []string{"On branch main", "Changes not staged for commit:"}
	for index := 0; index < 30; index++ {
		gitStatusLines = append(gitStatusLines, fmt.Sprintf("        modified:   internal/file_%02d.go", index))
	}
	gitLogLines := []string{}
	for index := 0; index < 40; index++ {
		gitLogLines = append(gitLogLines,
			fmt.Sprintf("commit %040x", index+1),
			"Author: Dev <dev@example.com>",
			"Date:   Mon Oct 1 00:00:00 2026 +0000",
			"",
			fmt.Sprintf("    subject number %d", index),
			"",
			"    long body paragraph that should be dropped by the filter",
			"    another long body paragraph that should also be dropped",
			" 3 files changed, 10 insertions(+), 2 deletions(-)",
			"diff --git a/a.go b/a.go",
		)
	}
	buildLines := []string{}
	for index := 0; index < 40; index++ {
		buildLines = append(buildLines, fmt.Sprintf("   Compiling crate_%02d v0.1.0", index))
	}
	buildLines = append(buildLines, "error[E0308]: mismatched types", " --> src/main.go:10:5", "   |", "10 |     let x: u8 = \"a\";", "   |            ^^^ expected `u8`, found `&str`", "", "error: could not compile `demo`", "Finished dev [unoptimized] target(s) in 1.2s")
	grepLines := []string{}
	for index := 0; index < 80; index++ {
		grepLines = append(grepLines, fmt.Sprintf("internal/file_%02d.go:%d:fmt.Println(\"hello\")", index%5, index))
	}
	findLines := []string{}
	for index := 0; index < 60; index++ {
		findLines = append(findLines, fmt.Sprintf("./internal/pkg_%02d/file_%02d.go", index%8, index))
	}
	lsLines := []string{"total 128"}
	for index := 0; index < 30; index++ {
		lsLines = append(lsLines, fmt.Sprintf("-rw-r--r-- 1 user group %d Oct 1 12:00 file_%02d.go", 100+index, index))
	}
	lsLines = append(lsLines, "drwxr-xr-x 2 user group 4096 Oct 1 12:00 cmd")
	treeLines := []string{".", "├── cmd", "│   └── api", "└── internal"}
	for index := 0; index < 220; index++ {
		treeLines = append(treeLines, fmt.Sprintf("    └── file_%03d.go", index))
	}
	dedupLines := []string{}
	for index := 0; index < 40; index++ {
		dedupLines = append(dedupLines, "noise line", "noise line", "noise line")
	}
	// smart-truncate só dispara quando a janela de detecção (1024 chars) tem
	// menos de 5 linhas não vazias (senão dedup-log vem antes) e o texto
	// completo passa de 250 linhas.
	smartLines := []string{}
	for index := 0; index < 390; index++ {
		smartLines = append(smartLines, "   ")
	}
	for index := 0; index < 100; index++ {
		smartLines = append(smartLines, fmt.Sprintf("plain unstructured line %d with some content", index))
	}
	// read-numbered exige >=250 linhas já na janela de 1024 chars, então as
	// linhas numeradas precisam ser curtas (4 bytes cada) para caberem.
	numberedLines := []string{}
	for index := 0; index < 400; index++ {
		numberedLines = append(numberedLines, "1|x")
	}
	searchLines := []string{"Result of search in '/repo' (total 40 files):"}
	for index := 0; index < 40; index++ {
		searchLines = append(searchLines, fmt.Sprintf("- internal/pkg_%02d/file_%02d.go", index%5, index))
	}

	cases := []struct {
		name   string
		filter string
		input  string
	}{
		{"git-diff", "git-diff", strings.Join(gitDiffLines, "\n")},
		{"git-status", "git-status", strings.Join(gitStatusLines, "\n")},
		{"git-log", "git-log", strings.Join(gitLogLines, "\n")},
		{"build-output", "build-output", strings.Join(buildLines, "\n")},
		{"grep", "grep", strings.Join(grepLines, "\n")},
		{"find", "find", strings.Join(findLines, "\n")},
		{"ls", "ls", strings.Join(lsLines, "\n")},
		{"tree", "tree", strings.Join(treeLines, "\n")},
		{"dedup-log", "dedup-log", strings.Join(dedupLines, "\n")},
		{"smart-truncate", "smart-truncate", strings.Join(smartLines, "\n")},
		{"read-numbered", "read-numbered", strings.Join(numberedLines, "\n")},
		{"search-list", "search-list", strings.Join(searchLines, "\n")},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			filter := autoDetectRTKFilter(testCase.input)
			if filter == nil {
				t.Fatalf("autodetect não encontrou filtro para %s", testCase.name)
			}
			if filter.name != testCase.filter {
				t.Fatalf("filtro=%q, esperado %q", filter.name, testCase.filter)
			}
			out := safeApplyRTK(filter, testCase.input)
			if out == "" {
				t.Fatalf("filtro %s devolveu vazio", filter.name)
			}
			if len(out) >= len(testCase.input) {
				t.Fatalf("filtro %s não reduziu (%d -> %d)", filter.name, len(testCase.input), len(out))
			}
		})
	}
}

func TestTokenSaverRTKPreservesClaudeErrorToolResult(t *testing.T) {
	payload := strings.Repeat("main.go:12:fmt.Println(\"hello\")\n", 80)
	body := map[string]any{"messages": []any{map[string]any{
		"role": "user",
		"content": []any{
			map[string]any{"type": "tool_result", "tool_use_id": "tool_1", "is_error": true, "content": payload},
			map[string]any{"type": "tool_result", "tool_use_id": "tool_2", "content": payload},
		},
	}}}
	stats := compressRTKMessages(body)
	if stats == nil || len(stats.Hits) != 1 {
		t.Fatalf("esperava exatamente uma compressão: %#v", stats)
	}
	blocks := body["messages"].([]any)[0].(map[string]any)["content"].([]any)
	if blocks[0].(map[string]any)["content"] != payload {
		t.Fatal("tool_result com is_error foi comprimido")
	}
	if blocks[1].(map[string]any)["content"] == payload {
		t.Fatal("tool_result normal não foi comprimido")
	}
}
