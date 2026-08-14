package tui

import (
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/bczaicki/vexillologist/internal/ld"
)

// flagItem is one row in the flag list. The per-environment state lives on
// flag.Environments; the matrix delegate holds the environment list itself.
type flagItem struct {
	flag   ld.FeatureFlag
	status string
}

func (i flagItem) FilterValue() string { return i.flag.Key + " " + i.flag.Name }

// flagList is the main screen: a full-width state matrix (one row per flag,
// one column per environment), with a detail view that replaces it on demand
// (enter to open, esc to close).
type flagList struct {
	list     list.Model
	delegate *matrixDelegate
	detail   viewport.Model
	// full is the last full-representation flag fetched with enter; the
	// detail view falls back to the summary of the selected row.
	full *ld.FeatureFlag
	// detailOpen swaps the list out for the detail view and routes navigation
	// keys to the viewport.
	detailOpen bool
	// statuses are LD evaluation statuses for the current env, keyed by flag
	// key; may be nil while loading or when unavailable.
	statuses      map[string]ld.FlagStatus
	archivedView  bool
	width, height int
}

func newFlagList() flagList {
	delegate := &matrixDelegate{}
	l := list.New(nil, delegate, 0, 0)
	l.SetShowTitle(false) // replaced by the matrix's own column header
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(true)
	l.DisableQuitKeybindings()
	l.SetShowHelp(false)
	// With the title hidden this bar is just the filter-input slot. Its default
	// style carries a bottom pad, which would make it two lines tall the moment
	// "/" is pressed and shift every row down; flatten it so the row positions
	// hold steady.
	l.Styles.TitleBar = lipgloss.NewStyle().PaddingLeft(gutterWidth)
	// ←/→ and h/l default to paging; the matrix needs them for the column
	// cursor. (This also frees u/d, which are the restore key and a scroll.)
	l.KeyMap.PrevPage = key.NewBinding(key.WithKeys("pgup", "b"), key.WithHelp("pgup", "prev page"))
	l.KeyMap.NextPage = key.NewBinding(key.WithKeys("pgdown", "f"), key.WithHelp("pgdn", "next page"))
	return flagList{list: l, delegate: delegate, detail: viewport.New(0, 0)}
}

// setEnvironments gives the matrix its columns and points the cursor at env.
func (f *flagList) setEnvironments(envs []ld.Environment, env string) {
	f.delegate.envs = envs
	f.focusEnv(env)
	f.applySize()
}

// focusEnv moves the column cursor to env, if present.
func (f *flagList) focusEnv(env string) {
	for i, e := range f.delegate.envs {
		if e.Key == env {
			f.delegate.cursor = i
			return
		}
	}
}

// moveEnv steps the column cursor by delta and returns the newly focused
// environment key ("" when there are no columns or the cursor did not move).
func (f *flagList) moveEnv(delta int) string {
	envs := f.delegate.envs
	if len(envs) == 0 {
		return ""
	}
	next := clamp(f.delegate.cursor+delta, 0, len(envs)-1)
	if next == f.delegate.cursor {
		return ""
	}
	f.delegate.cursor = next
	f.applySize() // the visible column window may need to scroll
	return envs[next].Key
}

// selectByKey moves the row cursor back to a flag key after a re-sort.
func (f *flagList) selectByKey(key string) {
	for i, item := range f.list.Items() {
		if fi, ok := item.(flagItem); ok && fi.flag.Key == key {
			f.list.Select(i)
			return
		}
	}
}

func (f *flagList) setFlags(flags []ld.FeatureFlag, env string, archivedView bool) {
	f.archivedView = archivedView
	items := make([]list.Item, len(flags))
	for i, flag := range flags {
		items[i] = flagItem{flag: flag, status: f.statusOf(flag.Key)}
	}
	f.list.SetItems(items)
	f.full = nil
	f.closeDetail()
	f.refreshDetail(env)
}

// setStatuses stores LD evaluation statuses and refreshes row annotations.
func (f *flagList) setStatuses(statuses map[string]ld.FlagStatus, env string) {
	f.statuses = statuses
	for i, item := range f.list.Items() {
		if fi, ok := item.(flagItem); ok {
			fi.status = f.statusOf(fi.flag.Key)
			f.list.SetItem(i, fi)
		}
	}
	f.refreshDetail(env)
}

func (f *flagList) statusOf(key string) string {
	if s, ok := f.statuses[key]; ok {
		return s.Name
	}
	return ""
}

// updateFlag replaces one flag in the list (after a toggle) in place.
func (f *flagList) updateFlag(updated *ld.FeatureFlag, env string) {
	items := f.list.Items()
	for i, item := range items {
		if fi, ok := item.(flagItem); ok && fi.flag.Key == updated.Key {
			// The semantic-patch response has full env configs; keep it.
			f.list.SetItem(i, flagItem{flag: *updated, status: f.statusOf(updated.Key)})
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

// openDetail swaps the list out for the detail view of the selected flag.
func (f *flagList) openDetail(env string) {
	f.detailOpen = true
	f.applySize()
	f.refreshDetail(env)
	f.detail.GotoTop()
}

func (f *flagList) closeDetail() {
	f.detailOpen = false
	f.applySize()
}

func (f *flagList) detailOpened() bool { return f.detailOpen }

func (f *flagList) setSize(w, h int) {
	f.width, f.height = w, h
	f.applySize()
}

// applySize gives the whole content area to whichever view is showing, and
// recomputes the matrix column budget for the new width.
func (f *flagList) applySize() {
	h := f.height - 1 // reserve a line for the key-binding help footer
	f.delegate.layout = computeLayout(f.width, len(f.delegate.envs), f.delegate.cursor)
	// The matrix header occupies one more line above the list.
	f.list.SetSize(f.width, max(1, h-1))
	// Account for the detail view's border and padding.
	f.detail.Width = max(1, f.width-4)
	f.detail.Height = max(1, h-2)
}

// refreshDetail re-renders the detail view for the current selection. It is a
// no-op while the list is showing, so cursor movement stays cheap.
func (f *flagList) refreshDetail(env string) {
	if !f.detailOpen {
		return
	}
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
	var status *ld.FlagStatus
	if s, ok := f.statuses[selected.Key]; ok {
		status = &s
	}
	f.detail.SetContent(renderFlagDetail(flag, env, f.detail.Width, fullyLoaded, status))
}

func (f *flagList) update(msg tea.Msg, env string) tea.Cmd {
	// Keys go to whichever view is showing; everything else goes to both.
	if _, isKey := msg.(tea.KeyMsg); isKey {
		var cmd tea.Cmd
		if f.detailOpen {
			f.detail, cmd = f.detail.Update(msg)
			return cmd
		}
		before := f.selected()
		f.list, cmd = f.list.Update(msg)
		if keyOf(before) != keyOf(f.selected()) {
			f.full = nil
		}
		return cmd
	}

	var cmds []tea.Cmd
	var cmd tea.Cmd
	f.list, cmd = f.list.Update(msg)
	cmds = append(cmds, cmd)
	f.detail, cmd = f.detail.Update(msg)
	cmds = append(cmds, cmd)
	return tea.Batch(cmds...)
}

func keyOf(f *ld.FeatureFlag) string {
	if f == nil {
		return ""
	}
	return f.Key
}

func (f *flagList) view() string {
	if f.detailOpen {
		return styleDetail.Render(f.detail.View()) + "\n" +
			styleHelpLine.Render(helpFooter(keys.detailHelp(f.archivedView)))
	}
	bindings := keys.shortHelp()
	if f.archivedView {
		bindings = keys.archivedHelp()
	}
	return f.delegate.header() + "\n" + f.list.View() + "\n" +
		styleHelpLine.Render(helpFooter(bindings))
}

// helpFooter renders key hints, e.g. "t toggle on/off · n new flag · …".
func helpFooter(bindings []key.Binding) string {
	var parts []string
	for _, binding := range bindings {
		h := binding.Help()
		parts = append(parts, h.Key+" "+h.Desc)
	}
	return strings.Join(parts, " · ")
}
