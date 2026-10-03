package convert

import (
	"math"
	"strconv"
	"strings"

	"ets2omsi/internal/pixtext"
	"ets2omsi/internal/scene"
)

// pitMaterial is one material of the selected ETS2 look as ConverterPIX
// writes it into the PIT ("Look { Name ... Material { Alias, Effect,
// Attribute { Tag: "diffuse" Value: ( &hex &hex &hex ) }, Texture { ... } } }").
type pitMaterial struct {
	Textures   []string
	Diffuse    [3]float64
	HasDiffuse bool
}

// parsePixFloats parses ConverterPIX float lists such as
// "( &3f800000  &3f4ccccd  &00000000 )" or plain decimals.
func parsePixFloats(s string) []float64 {
	s = strings.NewReplacer("(", " ", ")", " ", ",", " ").Replace(s)
	out := []float64{}
	for _, f := range strings.Fields(s) {
		if strings.HasPrefix(f, "&") {
			u, err := strconv.ParseUint(f[1:], 16, 32)
			if err != nil {
				return nil
			}
			out = append(out, float64(math.Float32frombits(uint32(u))))
			continue
		}
		v, err := strconv.ParseFloat(f, 64)
		if err != nil {
			return nil
		}
		out = append(out, v)
	}
	return out
}

func parsePITMaterial(s *pixtext.Section) (string, pitMaterial) {
	alias := strings.ToLower(strings.TrimSpace(pixtext.First(s, "Alias")))
	pm := pitMaterial{}
	type pair struct{ tag, val string }
	all := []pair{}
	primary := []string{}
	for _, t := range pixtext.Children(s, "Texture") {
		tag := strings.ToLower(strings.TrimSpace(pixtext.First(t, "Tag")))
		val := strings.TrimSpace(pixtext.First(t, "Value"))
		if val == "" {
			continue
		}
		all = append(all, pair{tag, val})
		if isVisibleBaseTextureTag(tag) {
			primary = append(primary, val)
		}
	}
	// Some very old traffic materials expose only one texture without a
	// modern texture_base tag. Accept it only when the tag is not clearly a
	// normal/mask/spec/reflection helper.
	if len(primary) == 0 && len(all) == 1 && !isNonDiffuseTextureTag(all[0].tag) {
		primary = append(primary, all[0].val)
	}
	pm.Textures = uniqueStringsLocal(primary)
	for _, a := range pixtext.Children(s, "Attribute") {
		if !strings.EqualFold(strings.TrimSpace(pixtext.First(a, "Tag")), "diffuse") {
			continue
		}
		if v := parsePixFloats(pixtext.First(a, "Value")); len(v) >= 3 {
			for i := 0; i < 3; i++ {
				pm.Diffuse[i] = math.Max(0, math.Min(1, v[i]))
			}
			pm.HasDiffuse = true
		}
	}
	return alias, pm
}

// pitLookMaterials returns the materials of one look. ConverterPIX gives every
// look the same aliases, so mixing looks (the old behaviour) let the last
// look's textures win. The wanted look is used when present, otherwise
// "default", otherwise the first look. PITs without Look sections fall back
// to every Material section.
func pitLookMaterials(file, look string) map[string]pitMaterial {
	secs, err := pixtext.ParseFile(file)
	if err != nil {
		return nil
	}
	looks := []*pixtext.Section{}
	var find func(*pixtext.Section)
	find = func(s *pixtext.Section) {
		if strings.EqualFold(s.Name, "Look") {
			looks = append(looks, s)
			return
		}
		for _, c := range s.Children {
			find(c)
		}
	}
	for _, s := range secs {
		find(s)
	}
	out := map[string]pitMaterial{}
	collect := func(s *pixtext.Section) {
		var walk func(*pixtext.Section)
		walk = func(x *pixtext.Section) {
			if strings.EqualFold(x.Name, "Material") {
				if alias, pm := parsePITMaterial(x); alias != "" {
					out[alias] = pm
				}
				return
			}
			for _, c := range x.Children {
				walk(c)
			}
		}
		walk(s)
	}
	if len(looks) == 0 {
		for _, s := range secs {
			collect(s)
		}
		return out
	}
	chosen := looks[0]
	want := strings.TrimSpace(look)
	for _, cand := range []string{want, "default"} {
		if cand == "" {
			continue
		}
		found := false
		for _, l := range looks {
			if strings.EqualFold(strings.TrimSpace(pixtext.First(l, "Name")), cand) {
				chosen, found = l, true
				break
			}
		}
		if found {
			break
		}
	}
	collect(chosen)
	return out
}

func pitHintsFromMaterials(mats map[string]pitMaterial) map[string][]string {
	out := map[string][]string{}
	for alias, pm := range mats {
		if len(pm.Textures) > 0 {
			out[alias] = pm.Textures
		}
	}
	return out
}

// loadPITMaterials applies the selected look's diffuse colours to the scene
// materials and returns that look's texture hints.
func loadPITMaterials(sc *scene.Scene, pitFile, look string) map[string][]string {
	if strings.TrimSpace(pitFile) == "" {
		return map[string][]string{}
	}
	mats := pitLookMaterials(pitFile, look)
	if sc != nil {
		for i := range sc.Materials {
			m := &sc.Materials[i]
			pm, ok := mats[strings.ToLower(strings.TrimSpace(m.Alias))]
			if !ok || !pm.HasDiffuse {
				continue
			}
			d := pm.Diffuse
			if d[0] > .97 && d[1] > .97 && d[2] > .97 {
				continue // neutral white: texture shows as-is
			}
			m.Tint, m.HasTint = d, true
		}
	}
	return pitHintsFromMaterials(mats)
}
