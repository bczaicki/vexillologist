package tui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/bczaicki/vexillologist/internal/ld"
	"github.com/bczaicki/vexillologist/internal/output"
)

// renderFlagDetail renders the right-hand detail pane for one flag. status is
// LD's evaluation status in env (nil when unavailable).
func renderFlagDetail(f *ld.FeatureFlag, env string, width int, fullyLoaded bool, status *ld.FlagStatus) string {
	var b strings.Builder
	wrap := lipgloss.NewStyle().Width(max(20, width))

	b.WriteString(styleTitle.Render(f.Key))
	if f.Archived {
		b.WriteString("  " + styleWarn.Render("⚑ ARCHIVED"))
	}
	b.WriteString("\n")
	b.WriteString(styleDim.Render("camelCase: "+kebabToCamel(f.Key)) + "\n\n")

	if f.Name != "" && f.Name != f.Key {
		b.WriteString(f.Name + "\n")
	}
	if f.Description != "" {
		b.WriteString(wrap.Render(f.Description) + "\n")
	}
	b.WriteString("\n")

	fmt.Fprintf(&b, "kind: %s   temporary: %v\n", f.Kind, f.Temporary)
	if f.CreationDate > 0 {
		fmt.Fprintf(&b, "created: %s ago\n", output.RelativeAge(f.CreationDate))
	}
	if status != nil {
		line := "status in " + env + ": " + renderStatusName(status.Name)
		if status.LastRequested != nil {
			line += styleDim.Render("  (last evaluated " + output.RelativeAge(status.LastRequested.UnixMilli()) + " ago)")
		}
		b.WriteString(line + "\n")
	}
	if len(f.Tags) > 0 {
		fmt.Fprintf(&b, "tags: %s\n", strings.Join(f.Tags, ", "))
	}
	if f.Maintainer != nil && f.Maintainer.Email != "" {
		fmt.Fprintf(&b, "maintainer: %s\n", f.Maintainer.Email)
	}

	if len(f.Variations) > 0 {
		b.WriteString("\n" + styleTitle.Render("variations") + "\n")
		for i, v := range f.Variations {
			name := ""
			if v.Name != "" {
				name = "  " + styleDim.Render(v.Name)
			}
			fmt.Fprintf(&b, "  [%d] %v%s\n", i, v.Value, name)
		}
	}

	if len(f.Environments) > 0 {
		b.WriteString("\n" + styleTitle.Render("environments") + "\n")
		envKeys := make([]string, 0, len(f.Environments))
		for key := range f.Environments {
			envKeys = append(envKeys, key)
		}
		sort.Strings(envKeys)
		for _, key := range envKeys {
			envCfg := f.Environments[key]
			state := styleOffDot.Render("off")
			if envCfg.On {
				state = styleOnDot.Render("on")
			}
			marker := "  "
			if key == env {
				marker = styleTitle.Render("▸ ")
			}
			age := ""
			if envCfg.LastModified > 0 {
				age = styleDim.Render("  changed " + output.RelativeAge(envCfg.LastModified) + " ago")
			}
			fmt.Fprintf(&b, "%s%-16s %s%s\n", marker, key, state, age)
			if key == env {
				b.WriteString(renderTargeting(envCfg, f.Variations))
			}
		}
	}

	if !fullyLoaded {
		b.WriteString("\n" + styleDim.Render("loading targeting details…"))
	}
	return b.String()
}

// renderStatusName colors an LD evaluation status by how strongly it
// suggests cleanup.
func renderStatusName(name string) string {
	switch name {
	case "inactive", "launched":
		return styleWarn.Render(name)
	case "active":
		return styleOnDot.Render(name)
	default:
		return name
	}
}

// renderTargeting summarizes an environment's targeting config (read-only).
func renderTargeting(envCfg ld.FlagEnvConfig, variations []ld.Variation) string {
	var b strings.Builder
	indent := "      "

	for _, target := range envCfg.Targets {
		fmt.Fprintf(&b, "%s%d individual target(s) → %s\n",
			indent, len(target.Values), variationLabel(variations, target.Variation))
	}
	for i, rule := range envCfg.Rules {
		desc := rule.Description
		if desc == "" {
			desc = fmt.Sprintf("rule %d (%d clause(s))", i+1, len(rule.Clauses))
		}
		serves := "rollout"
		if rule.Variation != nil {
			serves = variationLabel(variations, *rule.Variation)
		}
		fmt.Fprintf(&b, "%s%s → %s\n", indent, desc, serves)
	}
	if envCfg.Fallthrough != nil {
		serves := "rollout"
		if envCfg.Fallthrough.Variation != nil {
			serves = variationLabel(variations, *envCfg.Fallthrough.Variation)
		}
		fmt.Fprintf(&b, "%sfallthrough → %s\n", indent, serves)
	}
	if envCfg.OffVariation != nil {
		fmt.Fprintf(&b, "%swhen off → %s\n", indent, variationLabel(variations, *envCfg.OffVariation))
	}
	return b.String()
}

func variationLabel(variations []ld.Variation, idx int) string {
	if idx < 0 || idx >= len(variations) {
		return fmt.Sprintf("variation %d", idx)
	}
	v := variations[idx]
	if v.Name != "" {
		return v.Name
	}
	return fmt.Sprintf("%v", v.Value)
}

// kebabToCamel converts an LD flag key to the identifier the LaunchDarkly
// React SDK exposes via useFlags(), e.g. "connect-1-device-tag-filters" →
// "connect1DeviceTagFilters".
func kebabToCamel(key string) string {
	parts := strings.Split(key, "-")
	var b strings.Builder
	for i, part := range parts {
		if part == "" {
			continue
		}
		if i == 0 {
			b.WriteString(part)
			continue
		}
		b.WriteString(strings.ToUpper(part[:1]) + part[1:])
	}
	return b.String()
}
