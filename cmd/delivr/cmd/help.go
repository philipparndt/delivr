package cmd

import _ "embed"

//go:embed help/root.md
var helpRoot string

//go:embed help/generate.md
var helpGenerate string

//go:embed help/rotato.md
var helpRotato string

//go:embed help/frames.md
var helpFrames string

//go:embed help/frames-download.md
var helpFramesDownload string

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
	case "frames":
		return renderMarkdown(helpFrames)
	case "frames-download":
		return renderMarkdown(helpFramesDownload)
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
