package handlers

import _ "embed"

// Official compact rules, unmodified; not the client's hooks or slash commands.
// Source: DietrichGebert/ponytail@09c434ab73ca43761c5d8b0c5d341926c2fba710
//
//go:embed prompts/ponytail/AGENTS.md
var officialPonytailPrompt string

func init() {
	ponytailPrompts["official"] = officialPonytailPrompt
}
