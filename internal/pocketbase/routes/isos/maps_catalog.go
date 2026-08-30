// Maps-catalog view route (organizer redesign, Maps page). One GET returns
// everything the shelf + detail need: each canonical build (the `maps`
// collection) joined to the discs that carry it (via iso_maps rows matched on
// game+filename+hash) and to a BSP-render thumbnail fallback for builds with
// no uploaded graphic. Mutations don't live here — the page PATCHes the maps
// record through the PB SDK (organizer update rule + the maps_variant_guard
// hook); this view is just the read side, shaped once server-side so the
// client doesn't need isos list access to label disc chips.
package isos

import (
	"fmt"
	"net/http"
	"sort"
	"strings"

	"github.com/pocketbase/pocketbase/core"

	"github.com/Stewball32/xemu-cartographer/internal/isoingest"
)

func init() {
	register(registerMapsCatalog)
}

// catalogDisc is one carrying disc chip: {id, name}.
type catalogDisc struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// catalogView is one canonical build for the shelf + editor.
type catalogView struct {
	ID           string        `json:"id"`
	Game         string        `json:"game"`
	Filename     string        `json:"filename"`
	ContentHash  string        `json:"content_hash"`
	DisplayName  string        `json:"display_name"`
	VariantOf    string        `json:"variant_of"`
	Description  string        `json:"description"`
	PowerItems   any           `json:"power_items"`
	GraphicURL   string        `json:"graphic_url"`
	ThumbURL     string        `json:"thumb_url"`     // BSP-render fallback
	InternalName string        `json:"internal_name"` // cache-header name
	Discs        []catalogDisc `json:"discs"`
	Updated      string        `json:"updated"`
}

// GET /api/admin/isos/maps-catalog — every canonical build with its carriers.
func registerMapsCatalog() {
	Group.GET("/maps-catalog", func(e *core.RequestEvent) error {
		catalog, err := e.App.FindAllRecords("maps")
		if err != nil {
			return e.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
		}

		// Disc names + games, id-keyed.
		isoName := map[string]string{}
		isoGame := map[string]string{}
		if isos, err := e.App.FindAllRecords(collectionName); err == nil {
			for _, r := range isos {
				isoName[r.Id] = r.GetString("name")
				isoGame[r.Id] = isoingest.GameForTitleID(r.GetString("title_id"))
			}
		}

		// Per-build carriers + thumbnail fallback, keyed game|filename|hash.
		type buildFacts struct {
			discs    []catalogDisc
			thumbURL string
			internal string
		}
		facts := map[string]*buildFacts{}
		if rows, err := e.App.FindAllRecords("iso_maps"); err == nil {
			for _, r := range rows {
				isoID := r.GetString("iso")
				game := isoGame[isoID]
				hash := r.GetString("content_hash")
				if game == "" || hash == "" {
					continue
				}
				key := game + "|" + r.GetString("filename") + "|" + hash
				f := facts[key]
				if f == nil {
					f = &buildFacts{}
					facts[key] = f
				}
				f.discs = append(f.discs, catalogDisc{ID: isoID, Name: isoName[isoID]})
				if f.internal == "" {
					f.internal = r.GetString("name")
				}
				if f.thumbURL == "" && r.GetString("thumb_status") == "ready" {
					if fn := r.GetString("thumb"); fn != "" {
						f.thumbURL = fmt.Sprintf("/api/files/iso_maps/%s/%s", r.Id, fn)
					}
				}
			}
		}

		out := make([]catalogView, 0, len(catalog))
		for _, m := range catalog {
			v := catalogView{
				ID:          m.Id,
				Game:        m.GetString("game"),
				Filename:    m.GetString("filename"),
				ContentHash: m.GetString("content_hash"),
				DisplayName: m.GetString("display_name"),
				VariantOf:   m.GetString("variant_of"),
				Description: m.GetString("description"),
				Updated:     m.GetString("updated"),
				Discs:       []catalogDisc{},
			}
			var items any
			if err := m.UnmarshalJSONField("power_items", &items); err == nil && items != nil {
				v.PowerItems = items
			} else {
				v.PowerItems = []any{}
			}
			if fn := m.GetString("graphic"); fn != "" {
				v.GraphicURL = fmt.Sprintf("/api/files/maps/%s/%s", m.Id, fn)
			}
			key := v.Game + "|" + v.Filename + "|" + v.ContentHash
			if f := facts[key]; f != nil {
				sort.Slice(f.discs, func(i, j int) bool {
					return strings.ToLower(f.discs[i].Name) < strings.ToLower(f.discs[j].Name)
				})
				v.Discs = f.discs
				v.ThumbURL = f.thumbURL
				v.InternalName = f.internal
			}
			out = append(out, v)
		}
		sort.Slice(out, func(i, j int) bool {
			ni := strings.ToLower(out[i].DisplayName)
			if ni == "" {
				ni = strings.ToLower(out[i].Filename)
			}
			nj := strings.ToLower(out[j].DisplayName)
			if nj == "" {
				nj = strings.ToLower(out[j].Filename)
			}
			if ni != nj {
				return ni < nj
			}
			return out[i].ContentHash < out[j].ContentHash
		})
		return e.JSON(http.StatusOK, out)
	})
}
