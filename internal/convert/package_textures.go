package convert

import (
	"bytes"
	"path"
	"regexp"
	"strings"
)

// Texture name helpers shared by the resolver and the ConverterPIX-export
// index (see textures.go for the resolver itself).

var textureImageExts = []string{".dds", ".tga", ".png", ".jpg", ".jpeg", ".bmp"}

func isTextureImageExt(ext string) bool {
	ext = strings.ToLower(ext)
	for _, e := range textureImageExts {
		if e == ext {
			return true
		}
	}
	return false
}

func textureExtRank(ext string) int {
	ext = strings.ToLower(ext)
	for i, e := range textureImageExts {
		if e == ext {
			return i
		}
	}
	return len(textureImageExts)
}

// textureTightKey normalizes a virtual texture path so that names differing
// only in spaces/underscores/dashes/dots and extension compare equal:
// "/vehicle/x/Tableau de bord.dds" == "vehicle/x/tableaudebord".
func textureTightKey(p string) string {
	p = normalizeTextureKey(p)
	if ext := path.Ext(p); ext != "" && (isTextureImageExt(ext) || ext == ".tobj" || ext == ".mat") {
		p = strings.TrimSuffix(p, ext)
	}
	return strings.Map(func(r rune) rune {
		switch r {
		case ' ', '_', '-', '.', '\t':
			return -1
		}
		return r
	}, p)
}

func textureTightBase(p string) string {
	k := textureTightKey(p)
	if i := strings.LastIndex(k, "/"); i >= 0 {
		return k[i+1:]
	}
	return k
}

var tobjImagePath = regexp.MustCompile(`(?i)[\x20-\x7e]+?\.(dds|tga|png|jpe?g|bmp)`)

// tobjTexturePath returns the image path embedded in a binary/text TOBJ.
func tobjTexturePath(data []byte) string {
	for _, m := range tobjImagePath.FindAll(data, -1) {
		s := string(bytes.TrimSpace(m))
		if i := strings.Index(s, "/"); i >= 0 {
			s = s[i:]
		}
		if strings.TrimSpace(s) != "" {
			return s
		}
	}
	return ""
}
