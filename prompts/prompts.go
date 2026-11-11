// Package prompts holds the role templates. They are plain Markdown so
// they can be reviewed and reused like code; the harness embeds them.
package prompts

import _ "embed"

var (
	//go:embed shared.md
	Shared string
	//go:embed analyst.md
	Analyst string
	//go:embed architect.md
	Architect string
	//go:embed developer.md
	Developer string
	//go:embed qa.md
	QA string
	//go:embed docs.md
	Docs string
	//go:embed naive.md
	Naive string
)
