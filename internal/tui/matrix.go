package tui

import (
	"io"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/bczaicki/vexillologist/internal/ld"
)

// Column-width bounds for the state matrix. Env cells hold a state dot, so
// their width is really about keeping the header's env key readable.
const (
	gutterWidth = 2 // the list's selection marker column
	// minEnvCell fits "[●]" — a dot inside the column cursor's brackets —
	// with a space either side.
	minEnvCell  = 5
	maxEnvCell  = 10
	minKeyWidth = 24
	// maxKeyWidth caps the key column so a very wide terminal spends its space
	// on readable env labels and a ragged right edge, not one vast blank
	// column. The longest real LD key in use is 47 characters.
	maxKeyWidth = 48
	// keyWidthForStatus is the key width below which the status column is
	// dropped to buy back space.
	keyWidthForStatus = 30
	statusWidth       = 10
)

// matrixLayout is the resolved column geometry for one terminal width. first
// and count window the environment columns when they cannot all fit.
type matrixLayout struct {
	keyWidth   int
	envCell    int
	showStatus bool
	// first/count select the visible slice of environments.
	first, count int
}

// computeLayout budgets the row across key, env cells, and status. Priority
// when space is short: flag key > env cells > status.
func computeLayout(width int, envCount, cursor int) matrixLayout {
	l := matrixLayout{first: 0, count: envCount}
	avail := max(1, width-gutterWidth)

	if envCount == 0 {
		l.keyWidth = avail
		return l
	}

	// Start wide and shrink the env cells until the key column fits.
	l.envCell = maxEnvCell
	for l.envCell > minEnvCell && avail-envCount*l.envCell < minKeyWidth {
		l.envCell--
	}

	// Still too tight: show only the env columns that fit, windowed around the
	// cursor, with one cell each for the ‹ › overflow markers.
	if avail-envCount*l.envCell < minKeyWidth {
		l.count = max(1, (avail-minKeyWidth)/l.envCell)
		l.first = clamp(cursor-l.count/2, 0, max(0, envCount-l.count))
	}

	rest := avail - l.count*l.envCell
	if rest-statusWidth >= keyWidthForStatus {
		l.showStatus = true
		rest -= statusWidth
	}
	// Spend anything past the key cap on wider env cells (up to their label
	// width), then leave the rest as a ragged right edge.
	if slack := rest - maxKeyWidth; slack > 0 && l.count > 0 && l.envCell < maxEnvCell {
		grow := min(slack/l.count, maxEnvCell-l.envCell)
		l.envCell += grow
		rest -= grow * l.count
	}
	l.keyWidth = clamp(rest, 1, maxKeyWidth)
	return l
}

// hasLeftOverflow / hasRightOverflow report whether env columns are hidden
// off either side of the window.
func (l matrixLayout) hasLeftOverflow() bool { return l.first > 0 }
func (l matrixLayout) hasRightOverflow(envCount int) bool {
	return l.first+l.count < envCount
}

// matrixDelegate renders one flag per line as a row of per-environment
// on/off cells. It is held by pointer so envs and the column cursor can change
// without rebuilding every list item.
type matrixDelegate struct {
	envs   []ld.Environment
	cursor int
	layout matrixLayout
}

func (d *matrixDelegate) Height() int  { return 1 }
func (d *matrixDelegate) Spacing() int { return 0 }

func (d *matrixDelegate) Update(tea.Msg, *list.Model) tea.Cmd { return nil }

func (d *matrixDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	it, ok := item.(flagItem)
	if !ok {
		return
	}
	selected := index == m.Index()

	gutter := "  "
	keyStyle := styleRowKey
	if selected {
		gutter = styleTitle.Render("│") + " "
		keyStyle = styleRowKeySelected
	}

	name := it.flag.Key
	if it.flag.Archived {
		name += " ⚑"
	}
	var b strings.Builder
	b.WriteString(gutter)
	b.WriteString(cell(keyStyle.Render(truncateMiddle(name, d.layout.keyWidth)), d.layout.keyWidth, lipgloss.Left))
	b.WriteString(d.renderCells(it.flag))
	if d.layout.showStatus {
		b.WriteString(cell(renderStatusName(it.status), statusWidth, lipgloss.Left))
	}
	io.WriteString(w, b.String())
}

// renderCells renders the environment columns for one flag: a filled dot for
// on, a hollow one for off, and a dash where the flag has no config in that
// environment at all.
func (d *matrixDelegate) renderCells(flag ld.FeatureFlag) string {
	var b strings.Builder
	for i := d.layout.first; i < d.layout.first+d.layout.count && i < len(d.envs); i++ {
		glyph, style := "–", styleDim // no config in this environment
		if envCfg, ok := flag.Environments[d.envs[i].Key]; ok {
			if envCfg.On {
				glyph, style = "●", styleOnDot
			} else {
				glyph, style = "○", styleOffDot
			}
		}
		rendered := style.Render(glyph)
		if i == d.cursor {
			rendered = styleTitle.Render("[") + rendered + styleTitle.Render("]")
		}
		b.WriteString(cell(rendered, d.layout.envCell, lipgloss.Center))
	}
	return b.String()
}

// header renders the column-header line above the list, using the same widths
// and alignment as Render so the two can never drift apart.
func (d *matrixDelegate) header() string {
	var b strings.Builder
	b.WriteString(strings.Repeat(" ", gutterWidth))
	b.WriteString(cell(styleDim.Render("flag"), d.layout.keyWidth, lipgloss.Left))
	for i := d.layout.first; i < d.layout.first+d.layout.count && i < len(d.envs); i++ {
		// Head-truncate without an ellipsis: "produ" reads better than
		// "prod…" in a 5-wide cell, and the status bar always spells the
		// current environment out in full.
		label := headTruncate(d.envs[i].Key, d.layout.envCell-1)
		style := styleDim
		if i == d.cursor {
			style = styleTitle
		}
		// Overflow markers replace the outermost visible header cells.
		switch {
		case i == d.layout.first && d.layout.hasLeftOverflow():
			label = "‹" + label
		case i == d.layout.first+d.layout.count-1 && d.layout.hasRightOverflow(len(d.envs)):
			label += "›"
		}
		b.WriteString(cell(style.Render(label), d.layout.envCell, lipgloss.Center))
	}
	if d.layout.showStatus {
		b.WriteString(cell(styleDim.Render("status"), statusWidth, lipgloss.Left))
	}
	return b.String()
}

// cell pads an already-styled string to width by concatenating plain spaces.
//
// It deliberately does not use lipgloss's Width()/Align(): those *wrap* before
// they pad, so a cell one column too wide would silently become two lines —
// and a two-line row in a Height()==1 delegate desyncs the whole list
// viewport. lipgloss.Width measures printable width, so this stays correct
// through the ANSI escapes that len() would miscount.
func cell(s string, width int, align lipgloss.Position) string {
	gap := width - lipgloss.Width(s)
	if gap <= 0 {
		return s
	}
	if align == lipgloss.Center {
		left := gap / 2
		return strings.Repeat(" ", left) + s + strings.Repeat(" ", gap-left)
	}
	return s + strings.Repeat(" ", gap)
}

// headTruncate keeps the first width runes of s, with no ellipsis.
func headTruncate(s string, width int) string {
	runes := []rune(s)
	if width <= 0 || len(runes) <= width {
		return s
	}
	return string(runes[:width])
}

// truncateMiddle keeps both ends of a long flag key: the shared prefix
// identifies the family, the tail is what actually distinguishes it.
func truncateMiddle(s string, width int) string {
	runes := []rune(s)
	if width <= 0 || len(runes) <= width {
		return s
	}
	if width <= 3 {
		return headTruncate(s, width)
	}
	head := (width - 1) / 2
	tail := width - 1 - head
	return string(runes[:head]) + "…" + string(runes[len(runes)-tail:])
}

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
