package halosave

import (
	"embed"
	"fmt"
	"sort"
)

// templatesFS holds one real, clean save per (title, engine) so the
// template-patch generator always has a same-engine base to overwrite. These
// are genuine bytes pulled READ-ONLY from the fleet HDD images during the
// de-risk; they carry real (preserved) digests. See the package doc and the
// repo README for provenance.
//
//go:embed templates
var templatesFS embed.FS

// ceTemplatePaths maps a CE engine name to its embedded blam.lst template.
// One clean variant per engine (the 5 CE engines all had at least one sample).
var ceTemplatePaths = map[string]string{
	"slayer":  "templates/ce/slayer.blam.lst",  // from G-TS 50
	"ctf":     "templates/ce/ctf.blam.lst",     // from G-CTF 5C 10S
	"oddball": "templates/ce/oddball.blam.lst", // from G-BALL 5M 10S
	"king":    "templates/ce/king.blam.lst",    // from G-KOTH 5M 10S
	"race":    "templates/ce/race.blam.lst",    // from G-TS PRACTICE
}

// CEEngines returns the engine names for which a CE template exists, sorted.
func CEEngines() []string {
	out := make([]string, 0, len(ceTemplatePaths))
	for k := range ceTemplatePaths {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// CETemplate returns the embedded blam.lst template for the given engine name.
func CETemplate(engine string) ([]byte, error) {
	p, ok := ceTemplatePaths[engine]
	if !ok {
		return nil, fmt.Errorf("halosave: no CE template for engine %q (have %v)", engine, CEEngines())
	}
	return templatesFS.ReadFile(p)
}

// H2ProfileTemplate returns the embedded H2 player-profile template.
func H2ProfileTemplate() ([]byte, error) {
	return templatesFS.ReadFile("templates/h2/profile")
}

// H2GametypeTemplate returns the embedded H2 gametype template (mode = slayer;
// the only H2 gametype sample available). The payload-file name a LAN client
// should write is H2GametypeMode.
func H2GametypeTemplate() ([]byte, error) {
	return templatesFS.ReadFile("templates/h2/gametype")
}

// H2GametypeMode is the in-engine mode name of the embedded H2 gametype
// template, which is also the FATX payload filename Halo expects.
const H2GametypeMode = "slayer"
