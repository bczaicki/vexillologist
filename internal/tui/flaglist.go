package tui

import (
	"strings"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/phillips-connect/vexillologist/internal/ld"
)

// flagItem is one row in the flag list.
type flagItem struct {
	flag ld.FeatureFlag
	env  string
}

func (i flagItem) Title() string {
	dot := styleOffDot.Render("●")
	if envCfg, ok := i.flag.Environments[i.env]; ok && envCfg.On {
		dot = styleOnDot.Render("●")
	}
	return dot + " " + i.flag.Key
}

func (i flagItem) Description() string {
	desc := i.flag.Name
	if len(i.flag.Tags) > 0 {
		desc += "  " + styleDim.Render("["+strings.Join(i.flag.Tags, ", ")+"]")
	}
	return desc
}

func (i flagItem) FilterValue() string { return i.flag.Key + " " + i.flag.Name }

// flagList is the main screen: a filterable list on the left, a detail pane
// on the right.
type flagList struct {
	list   list.Model
	detail viewport.Model
	// full is the last full-representation flag fetched with enter; the
	// detail pane falls back to the summary of the selected row.
	full          *ld.FeatureFlag
	width, height int
}

func newFlagList() flagList {
	delegate := list.NewDefaultDelegate()
	delegate.SetSpacing(0)
	l := list.New(nil, delegate, 0, 0)
	l.Title = "flags"
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(true)
	l.DisableQuitKeybindings()
	l.SetShowHelp(false)
	return flagList{list: l, detail: viewport.New(0, 0)}
}

func (f *flagList) setFlags(flags []ld.FeatureFlag, env string) {
	items := make([]list.Item, len(flags))
	for i, flag := range flags {
		items[i] = flagItem{flag: flag, env: env}
	}
	f.list.SetItems(items)
	f.full = nil
	f.refreshDetail(env)
}

// updateFlag replaces one flag in the list (after a toggle) in place.
func (f *flagList) updateFlag(updated *ld.FeatureFlag, env string) {
	items := f.list.Items()
	for i, item := range items {
		if fi, ok := item.(flagItem); ok && fi.flag.Key == updated.Key {
			// The semantic-patch response has full env configs; keep it.
			f.list.SetItem(i, flagItem{flag: *updated, env: env})
			break
		}
	}
	if f.full != nil && f.full.Key == updated.Key {
		f.full = updated
	}
	f.refreshDetail(env)
}

// removeFlag drops a flag from the list (after archiving).
func (f *flagList) removeFlag(key, env string) {
	for i, item := range f.list.Items() {
		if fi, ok := item.(flagItem); ok && fi.flag.Key == key {
			f.list.RemoveItem(i)
			break
		}
	}
	if f.full != nil && f.full.Key == key {
		f.full = nil
	}
	f.refreshDetail(env)
}

// selected returns the highlighted flag, or nil when the list is empty.
func (f *flagList) selected() *ld.FeatureFlag {
	item, ok := f.list.SelectedItem().(flagItem)
	if !ok {
		return nil
	}
	return &item.flag
}

func (f *flagList) setFullDetail(flag *ld.FeatureFlag, env string) {
	f.full = flag
	f.refreshDetail(env)
}

func (f *flagList) filtering() bool {
	return f.list.FilterState() == list.Filtering
}

func (f *flagList) setSize(w, h int) {
	f.width, f.height = w, h
	h-- // reserve a line for the key-binding help footer
	listWidth := w * 2 / 5
	f.list.SetSize(listWidth, h)
	// Account for the detail pane's border and padding.
	f.detail.Width = w - listWidth - 4
	f.detail.Height = h - 2
}

// refreshDetail re-renders the detail pane for the current selection.
func (f *flagList) refreshDetail(env string) {
	selected := f.selected()
	if selected == nil {
		f.detail.SetContent(styleDim.Render("no flag selected"))
		return
	}
	flag := selected
	fullyLoaded := false
	if f.full != nil && f.full.Key == selected.Key {
		flag = f.full
		fullyLoaded = true
	}
	f.detail.SetContent(renderFlagDetail(flag, env, f.detail.Width, fullyLoaded))
}

func (f *flagList) update(msg tea.Msg, env string) tea.Cmd {
	var cmds []tea.Cmd
	before := f.selected()

	var cmd tea.Cmd
	f.list, cmd = f.list.Update(msg)
	cmds = append(cmds, cmd)

	f.detail, cmd = f.detail.Update(msg)
	cmds = append(cmds, cmd)

	after := f.selected()
	if keyOf(before) != keyOf(after) {
		f.full = nil
		f.refreshDetail(env)
	}
	return tea.Batch(cmds...)
}

func keyOf(f *ld.FeatureFlag) string {
	if f == nil {
		return ""
	}
	return f.Key
}

func (f *flagList) view() string {
	columns := lipgloss.JoinHorizontal(lipgloss.Top,
		f.list.View(),
		styleDetail.Render(f.detail.View()),
	)
	return columns + "\n" + styleHelpLine.Render(helpFooter())
}

// helpFooter renders the flag-list key hints, e.g. "t toggle on/off · n new flag · …".
func helpFooter() string {
	var parts []string
	for _, binding := range keys.shortHelp() {
		h := binding.Help()
		parts = append(parts, h.Key+" "+h.Desc)
	}
	return strings.Join(parts, " · ")
}
