package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/bczaicki/vexillologist/internal/ld"
)

// The flag-wide settings LaunchDarkly groups under "Advanced controls". The
// create form and the settings modal both show them as checkbox rows.
const (
	settingClientSide = iota
	settingMobile
	settingPermanent
	settingCount
)

var settingLabels = [settingCount]string{
	"available on client-side SDKs",
	"available on mobile SDKs",
	"permanent flag",
}

// settingValues holds one on/off per setting row.
type settingValues [settingCount]bool

// defaultSettings are a new flag's: client-side SDKs on, mobile off, temporary.
var defaultSettings = settingValues{settingClientSide: true}

// settingsOf reads a flag's current settings. SDK availability the API didn't
// return reads as off.
func settingsOf(f *ld.FeatureFlag) settingValues {
	var v settingValues
	if a := f.ClientSideAvailability; a != nil {
		v[settingClientSide] = a.UsingEnvironmentID
		v[settingMobile] = a.UsingMobileKey
	}
	v[settingPermanent] = !f.Temporary
	return v
}

func (v settingValues) availability() *ld.ClientSideAvailability {
	return &ld.ClientSideAvailability{
		UsingEnvironmentID: v[settingClientSide],
		UsingMobileKey:     v[settingMobile],
	}
}

// diff is the update that turns old into v, setting only what changed.
func (v settingValues) diff(old settingValues) ld.FlagSettingsUpdate {
	changed := func(i int) *bool {
		if v[i] == old[i] {
			return nil
		}
		on := v[i]
		return &on
	}
	return ld.FlagSettingsUpdate{
		ClientSide: changed(settingClientSide),
		Mobile:     changed(settingMobile),
		Permanent:  changed(settingPermanent),
	}
}

// settingRow renders one checkbox row, e.g. "> [x] available on client-side SDKs".
func settingRow(i int, on, focused bool) string {
	cursor := "  "
	if focused {
		cursor = styleTitle.Render("> ")
	}
	box := styleOffDot.Render("[ ]")
	if on {
		box = styleOnDot.Render("[x]")
	}
	return cursor + box + " " + settingLabels[i]
}

// settingsModal edits the selected flag's settings. Applying hands back only
// what changed; the caller gates it behind a confirm modal.
type settingsModal struct {
	flagKey string
	orig    settingValues
	values  settingValues
	focus   int
}

func newSettingsModal(f *ld.FeatureFlag) *settingsModal {
	v := settingsOf(f)
	return &settingsModal{flagKey: f.Key, orig: v, values: v}
}

// update returns (done, update). done with a nil update means the modal was
// cancelled or applied without changes.
func (m *settingsModal) update(msg tea.KeyMsg) (bool, *ld.FlagSettingsUpdate) {
	switch msg.String() {
	case "esc":
		return true, nil
	case "up", "k", "shift+tab":
		m.focus = (m.focus + settingCount - 1) % settingCount
	case "down", "j", "tab":
		m.focus = (m.focus + 1) % settingCount
	case " ", "x":
		m.values[m.focus] = !m.values[m.focus]
	case "enter":
		u := m.values.diff(m.orig)
		if u.Empty() {
			return true, nil
		}
		return true, &u
	}
	return false, nil
}

func (m *settingsModal) view() string {
	var b strings.Builder
	b.WriteString(styleTitle.Render("settings — "+m.flagKey) + "\n")
	b.WriteString(styleDim.Render("flag-wide: applies to ALL environments") + "\n\n")
	for i, on := range m.values {
		b.WriteString(settingRow(i, on, i == m.focus) + "\n")
	}
	b.WriteString("\n" + styleDim.Render("↑/↓ move · space toggles · enter applies · esc cancels"))
	return styleModal.Render(b.String())
}
