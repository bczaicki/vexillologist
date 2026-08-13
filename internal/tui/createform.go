package tui

import (
	"regexp"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/phillips-connect/vexillologist/internal/ld"
)

const (
	fieldName = iota
	fieldKey
	fieldDescription
	fieldTags
	fieldCount
)

// createForm is the new-flag screen: name, key (auto-slugified from name
// until edited), description, tags.
type createForm struct {
	inputs    [fieldCount]textinput.Model
	focus     int
	keyEdited bool
	errText   string
}

func newCreateForm() createForm {
	f := createForm{}
	labels := [fieldCount]string{"Name", "Key", "Description", "Tags (comma-separated)"}
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
		}
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
		Key:         key,
		Name:        name,
		Description: strings.TrimSpace(f.inputs[fieldDescription].Value()),
		Tags:        tags,
	}, true
}

func (f *createForm) setFocus(i int) {
	f.inputs[f.focus].Blur()
	f.focus = i
	f.inputs[i].Focus()
	f.errText = ""
}

func (f *createForm) view() string {
	var b strings.Builder
	b.WriteString(styleTitle.Render("create flag") + "\n\n")
	labels := [fieldCount]string{"name", "key", "description", "tags"}
	for i, input := range f.inputs {
		cursor := "  "
		if i == f.focus {
			cursor = styleTitle.Render("> ")
		}
		b.WriteString(cursor + styleDim.Render(labels[i]) + "\n" + cursor + input.View() + "\n\n")
	}
	if f.errText != "" {
		b.WriteString(styleError.Render(f.errText) + "\n")
	}
	b.WriteString(styleHelpLine.Render("tab/enter next · shift+tab back · enter on tags submits · esc cancels"))
	return b.String()
}
