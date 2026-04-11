package cmd

import _ "embed"

//go:embed help/root.md
var helpRoot string

//go:embed help/generate.md
var helpGenerate string

//go:embed help/rotato.md
var helpRotato string

//go:embed help/rotato-frame.md
var helpRotatoFrame string

//go:embed help/deliver.md
var helpDeliver string

//go:embed help/version.md
var helpVersion string

//go:embed help/completion.md
var helpCompletion string

func GetHelp(topic string) string {
	switch topic {
	case "root":
		return renderMarkdown(helpRoot)
	case "generate":
		return renderMarkdown(helpGenerate)
	case "rotato":
		return renderMarkdown(helpRotato)
	case "rotato-frame":
		return renderMarkdown(helpRotatoFrame)
	case "deliver":
		return renderMarkdown(helpDeliver)
	case "version":
		return renderMarkdown(helpVersion)
	case "completion":
		return renderMarkdown(helpCompletion)
	default:
		return ""
	}
}
