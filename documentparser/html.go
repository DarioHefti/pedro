package documentparser

import (
	"os"
	"strings"

	"github.com/JohannesKaufmann/html-to-markdown/v2/converter"
	"github.com/JohannesKaufmann/html-to-markdown/v2/plugin/base"
	"github.com/JohannesKaufmann/html-to-markdown/v2/plugin/commonmark"
	"github.com/JohannesKaufmann/html-to-markdown/v2/plugin/table"
)

func extractHTML(path string) (*Result, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	stat, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	fileSizeMB := float64(stat.Size()) / (1024 * 1024)

	conv := converter.NewConverter(converter.WithPlugins(
		base.NewBasePlugin(), // already drops script, style, noscript, iframe, ...
		commonmark.NewCommonmarkPlugin(),
		table.NewTablePlugin(table.WithCellPaddingBehavior(table.CellPaddingBehaviorMinimal)),
	))
	for _, tag := range []string{"nav", "header", "footer", "aside", "svg", "canvas"} {
		conv.Register.TagType(tag, converter.TagTypeRemove, converter.PriorityStandard)
	}
	md, err := conv.ConvertString(string(b))
	if err != nil {
		return nil, err
	}
	lines := strings.Split(strings.ReplaceAll(md, "\r\n", "\n"), "\n")
	var out []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			out = append(out, line)
		}
	}
	if len(out) == 0 {
		out = []string{"(No text content extracted from HTML)"}
	}
	return &Result{
		Format:     "html",
		FileSizeMB: fileSizeMB,
		Lines:      out,
	}, nil
}
