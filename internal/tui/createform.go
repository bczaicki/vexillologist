package tui

import (
	"regexp"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/bczaicki/vexillologist/internal/ld"
)

const (
	fieldName = iota
	fieldKey
	fieldDescription
	fieldTags
	// textFieldCount is the number of text inputs; the setting rows follow.
	textFieldCount
	fieldCount = textFieldCount + settingCount
)

// createForm is the new-flag screen: name, key (auto-slugified from name
// until edited), description, tags, then the flag-wide settings.
type createForm struct {
	inputs    [textFieldCount]textinput.Model
	settings  settingValues
	focus     int
	keyEdited bool
	errText   string
}

func newCreateForm() createForm {
	f := createForm{settings: defaultSettings}
	labels := [textFieldCount]string{"Name", "Key", "Description", "Tags (comma-separated)"}
	for i := range f.inputs {
		ti := textinput.New()
		ti.Placeholder = labels[i]
		ti.CharLimit = 256
		ti.Width = 48
		f.inputs[i] = ti
	}
	f.inputs[fieldName].Focus()
	return f
}

var nonSlug = regexp.MustCompile(`[^a-z0-9]+`)

// slugify derives a kebab-case flag key from a human name.
func slugify(name string) string {
	s := nonSlug.ReplaceAllString(strings.ToLower(name), "-")
	return strings.Trim(s, "-")
}

// update returns (submitted params, done, cmd). done with nil params means
// the form was cancelled.
func (f *createForm) update(msg tea.Msg) (*ld.CreateFlagParams, bool, tea.Cmd) {
	keyMsg, isKey := msg.(tea.KeyMsg)
	if isKey {
		switch keyMsg.String() {
		case "esc":
			return nil, true, nil
		case "tab", "down":
			f.setFocus((f.focus + 1) % fieldCount)
			return nil, false, nil
		case "shift+tab", "up":
			f.setFocus((f.focus + fieldCount - 1) % fieldCount)
			return nil, false, nil
		case "enter":
			if f.focus < fieldCount-1 {
				f.setFocus(f.focus + 1)
				return nil, false, nil
			}
			params, ok := f.buildParams()
			if !ok {
				return nil, false, nil
			}
			return params, true, nil
		case " ":
			if f.focus >= textFieldCount {
				i := f.focus - textFieldCount
				f.settings[i] = !f.settings[i]
				return nil, false, nil
			}
		}
	}

	// Setting rows have no input to forward to.
	if f.focus >= textFieldCount {
		return nil, false, nil
	}
	var cmd tea.Cmd
	f.inputs[f.focus], cmd = f.inputs[f.focus].Update(msg)

	if f.focus == fieldKey && isKey && keyMsg.Type == tea.KeyRunes {
		f.keyEdited = true
	}
	// Mirror the name into the key until the user edits the key directly.
	if f.focus == fieldName && !f.keyEdited {
		f.inputs[fieldKey].SetValue(slugify(f.inputs[fieldName].Value()))
	}
	return nil, false, cmd
}

func (f *createForm) buildParams() (*ld.CreateFlagParams, bool) {
	key := strings.TrimSpace(f.inputs[fieldKey].Value())
	name := strings.TrimSpace(f.inputs[fieldName].Value())
	if key == "" {
		f.errText = "key is required"
		f.setFocus(fieldKey)
		return nil, false
	}
	if name == "" {
		name = key
	}
	var tags []string
	for _, tag := range strings.Split(f.inputs[fieldTags].Value(), ",") {
		if tag = strings.TrimSpace(tag); tag != "" {
			tags = append(tags, tag)
		}
	}
	return &ld.CreateFlagParams{
		Key:                    key,
		Name:                   name,
		Description:            strings.TrimSpace(f.inputs[fieldDescription].Value()),
		Tags:                   tags,
		Temporary:              !f.settings[settingPermanent],
		ClientSideAvailability: f.settings.availability(),
	}, true
}

func (f *createForm) setFocus(i int) {
	if f.focus < textFieldCount {
		f.inputs[f.focus].Blur()
	}
	f.focus = i
	if i < textFieldCount {
		f.inputs[i].Focus()
	}
	f.errText = ""
}

func (f *createForm) view() string {
	var b strings.Builder
	b.WriteString(styleTitle.Render("create flag") + "\n\n")
	labels := [textFieldCount]string{"name", "key", "description", "tags"}
	for i, input := range f.inputs {
		cursor := "  "
		if i == f.focus {
			cursor = styleTitle.Render("> ")
		}
		b.WriteString(cursor + styleDim.Render(labels[i]) + "\n" + cursor + input.View() + "\n\n")
	}
	b.WriteString("  " + styleDim.Render("advanced controls") + "\n")
	for i, on := range f.settings {
		b.WriteString(settingRow(i, on, f.focus == textFieldCount+i) + "\n")
	}
	b.WriteString("\n")
	if f.errText != "" {
		b.WriteString(styleError.Render(f.errText) + "\n")
	}
	b.WriteString(styleHelpLine.Render("tab/enter next · shift+tab back · space toggles · enter on the last row submits · esc cancels"))
	return b.String()
}
