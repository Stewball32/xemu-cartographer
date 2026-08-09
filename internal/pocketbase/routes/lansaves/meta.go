package lansaves

import (
	"net/http"

	"github.com/pocketbase/pocketbase/core"

	"github.com/Stewball32/xemu-cartographer/internal/halosave"
)

// GET /api/lan/saves/meta — describes the generator's capabilities so the
// editors can render dynamically: available titles/kinds, CE engines, the H2
// appearance field list, and the current digest status. No content is produced.
func init() {
	register(func() {
		Group.GET("/meta", func(e *core.RequestEvent) error {
			type field struct {
				Offset int    `json:"offset"`
				Key    string `json:"key"`
				Label  string `json:"label"`
			}
			h2Fields := make([]field, 0, len(halosave.H2ProfileFields))
			for _, f := range halosave.H2ProfileFields {
				h2Fields = append(h2Fields, field{f.Offset, f.Key, f.Label})
			}
			// per-engine score-limit unit word (Slayer kills / CTF captures / …)
			ceScoreUnits := map[string]string{}
			for _, eng := range halosave.CEEngines() {
				ceScoreUnits[eng] = halosave.CEScoreUnit(eng)
			}
			return e.JSON(http.StatusOK, map[string]any{
				"titles": []map[string]any{
					{
						"id":       halosave.TitleCE,
						"label":    "Halo: Combat Evolved",
						"title_id": halosave.TitleIDHaloCE,
						"kinds":    []string{halosave.KindGametype, halosave.KindProfile},
					},
					{
						"id":       halosave.TitleH2,
						"label":    "Halo 2",
						"title_id": halosave.TitleIDHalo2,
						"kinds":    []string{halosave.KindProfile, halosave.KindGametype},
					},
				},
				"ce_engines": halosave.CEEngines(),
				// CE editor schemas — the UI renders itself from these.
				"ce_gametype_fields":   halosave.CEGametypeSchema(),
				"ce_gametype_sections": halosave.CESections(),
				"ce_profile_fields":    halosave.CEProfileSchema(),
				"ce_profile_sections":  halosave.CEProfileSections(),
				"ce_score_units":       ceScoreUnits,
				"h2_appearance":        h2Fields,
				"h2_gametype_mode":     halosave.H2GametypeMode,
				"fatx_cluster":         cfg.FATXCluster,
				"digest_resolved":      halosave.DigestResolved(),
				"digest_note":          halosave.DigestNote,
				"formats":              []string{"tar", "zip", "payload", "savemeta", "file"},
			})
		})
	})
}
