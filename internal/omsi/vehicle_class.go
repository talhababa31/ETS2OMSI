package omsi

import "strings"

// Vehicle classes used for physics, AI speed and the OMSI friendly name.
const (
	ClassSedan     = "sedan"
	ClassHatchback = "hatchback"
	ClassWagon     = "kombi"
	ClassCoupe     = "coupe"
	ClassSUV       = "suv"
	ClassPickup    = "pickup"
	ClassVan       = "van"
	ClassMinibus   = "minibus"
)

// ClassInfo holds the per-class tuning.
type ClassInfo struct {
	ID        string  `json:"id"`
	Label     string  `json:"label"`
	MassMult  float64 `json:"-"` // multiplier on the envelope-volume mass estimate
	MinMass   float64 `json:"-"`
	MaxMass   float64 `json:"-"`
	RideHz    float64 `json:"-"` // suspension natural frequency
	VMax      int     `json:"-"` // AI top speed km/h
	FrontLoad float64 `json:"-"`
}

var classTable = []ClassInfo{
	{ClassSedan, "Sedan", 1.00, 1.10, 2.10, 2.3, 170, .56},
	{ClassHatchback, "Hatchback", .96, .85, 1.60, 2.4, 160, .60},
	{ClassWagon, "Kombi (station wagon)", 1.02, 1.20, 2.20, 2.25, 165, .55},
	{ClassCoupe, "Coupe / spor", .98, 1.10, 2.00, 2.6, 190, .54},
	{ClassSUV, "SUV / crossover", 1.17, 1.45, 3.00, 2.0, 160, .54},
	{ClassPickup, "Pickup", 1.20, 1.70, 3.00, 1.9, 150, .58},
	{ClassVan, "Van / panelvan", 1.22, 1.65, 3.50, 1.85, 135, .52},
	{ClassMinibus, "Minibüs", 1.30, 2.20, 4.50, 1.8, 120, .50},
}

// Classes lists every class for the UI.
func Classes() []ClassInfo { return append([]ClassInfo(nil), classTable...) }

// LookupClass returns the class info for an id ("" or unknown -> sedan, false).
func LookupClass(id string) (ClassInfo, bool) {
	id = strings.ToLower(strings.TrimSpace(id))
	for _, c := range classTable {
		if c.ID == id {
			return c, true
		}
	}
	return classTable[0], false
}

func hasAny(s string, words ...string) bool {
	for _, w := range words {
		if strings.Contains(s, w) {
			return true
		}
	}
	return false
}

// ClassFromName recognises the class from the vehicle's display name / id.
func ClassFromName(name string) string {
	n := " " + strings.ToLower(name) + " "
	switch {
	case hasAny(n, "minibus", "minibüs", "bus ", "shuttle"):
		return ClassMinibus
	case hasAny(n, "sprinter", "transit", "ducato", "boxer", "jumper", "crafter", "master", "movano", "vivaro", "trafic", "vito", "transporter", "multivan", "caravelle", "daily", "iveco", "jumpy", "expert", "scudo", "proace", "vw_lt", " lt ", "lt35", "kangoo", "caddy", "berlingo", "partner", "doblo", "combo", "connect", "van"):
		return ClassVan
	case hasAny(n, "pickup", "pick-up", "pick up", "hilux", "amarok", "ranger", "l200", "navara", "d-max", "dmax", "tundra", "f-150", "f150", "raptor", "ram 1500"):
		return ClassPickup
	case hasAny(n, " x1", " x3", " x4", " x5", " x6", " x7", "q3", "q5", "q7", "q8", "glk", "glc", " gle", " gls", " ml ", "tiguan", "touareg", "rav4", "cr-v", "crv", "x-trail", "qashqai", "sorento", "sportage", "kuga", "pajero", "cherokee", "duster", "santa fe", "xc60", "xc90", "cayenne", "range rover", "land cruiser", "escalade", "tahoe", "suburban", "suv", "4x4", "jeep"):
		return ClassSUV
	case hasAny(n, "kombi", "estate", "wagon", "touring", "variant", "avant", " sw ", "break", "caravan", "sportbrake", "tourer"):
		return ClassWagon
	case hasAny(n, "coupe", "coupé", "cabrio", "roadster", "spider", " gt "):
		return ClassCoupe
	case hasAny(n, " hb ", "hatch", " ka ", "fiesta", "polo", " golf", "clio", "corsa", "punto", "yaris", "micra", " 206", " 207", " 208", " c3", "ibiza", "i10", "i20", " up ", "aygo", "twingo", "panda", " 500", "fabia", "mini "):
		return ClassHatchback
	case hasAny(n, "sedan", "limousine", "saloon"):
		return ClassSedan
	}
	return ""
}

// EstimateAIPhysicsClass is EstimateAIPhysics with an explicit class.
func EstimateAIPhysicsClass(name, class string, length, width, height float64) PhysicsProfile {
	c, ok := LookupClass(class)
	if !ok {
		return EstimateAIPhysics(name, length, width, height)
	}
	p := EstimateAIPhysics(name, length, width, height)
	l, w, h := safeDim(length, 4.5), safeDim(width, 1.8), safeDim(height, 1.5)
	mass := 0.122 * l * w * h * c.MassMult
	if mass < c.MinMass {
		mass = c.MinMass
	}
	if mass > c.MaxMass {
		mass = c.MaxMass
	}
	// keep explicit model anchors (F30, X6 ...) from the name estimate
	if strings.Contains(p.Profile, "-class") {
		mass = p.Mass
	}
	p.Mass = float64(int(mass*100+.5)) / 100
	p.Profile = c.Label
	p.Class = c.ID
	return p
}
