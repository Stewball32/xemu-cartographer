package haloce

import (
	"encoding/hex"
	"fmt"

	"github.com/Stewball32/xemu-cartographer/internal/scraper"
)

// BuildScoreProbe samples every address known to be involved in gametype
// detection, team-score lookup, score-limit lookup, and per-player score
// reads. The returned bag is rendered verbatim by the debug page's Probe
// tab so a human can spot which raw value matches what they see in-game —
// useful while the canonical offsets are still being worked out.
//
// All reads are best-effort: failures are silently dropped (the bag just
// won't contain that key), so a partial probe is still useful when memory
// hasn't fully initialised yet.
func (r *Reader) BuildScoreProbe() scraper.ScoreProbe {
	out := scraper.ScoreProbe{}

	out["gametype_candidates"] = r.probeGametypeCandidates()
	out["team_scores_raw"] = r.probeTeamScoresRaw()
	out["score_limits_raw"] = r.probeScoreLimitsRaw()
	out["per_player_score_tables"] = r.probePerPlayerScoreTables()
	out["per_player_static_struct"] = r.probePerPlayerStaticStruct()

	return out
}

func (r *Reader) probeGametypeCandidates() map[string]any {
	out := map[string]any{}

	// Original (wrong) approach — variant byte at AddrVariant.
	if hva, err := r.inst.LowHVA(AddrVariant); err == nil {
		if v, err := r.inst.Mem.ReadU8At(hva); err == nil {
			out["variant_u8_at_2f90f4"] = v
		}
	}

	// Engine-globals pointer + a sweep of nearby offsets read as both u32
	// and u8 — the gametype field is supposedly at +0x04 per legacy, but
	// reading 0 there suggests we should look elsewhere.
	gePtr, err := r.inst.DerefLowPtr(AddrGameEngineGlobalsPtr)
	if err == nil {
		out["ge_globals_ptr"] = fmt.Sprintf("0x%08x", gePtr)
		out["ge_globals_ptr_valid"] = gePtr >= HighGVAThreshold
		if gePtr >= HighGVAThreshold {
			for _, off := range []uint32{0x00, 0x04, 0x08, 0x0C, 0x10, 0x14, 0x18, 0x1C, 0x20} {
				if v, err := r.inst.Mem.ReadU32(gePtr + off); err == nil {
					out[fmt.Sprintf("ge_plus_%02x_u32", off)] = v
				}
				if v, err := r.inst.Mem.ReadU8(gePtr + off); err == nil {
					out[fmt.Sprintf("ge_plus_%02x_u8", off)] = v
				}
			}
			// Hexdump the first 64 bytes so the user can spot a small
			// integer that matches the running gametype.
			if b, err := r.inst.Mem.ReadBytes(gePtr, 64); err == nil {
				out["ge_globals_first_64_bytes_hex"] = hex.EncodeToString(b)
			}
		}
	}

	// Other addresses the legacy halocaster.py touched in the gametype area.
	for _, c := range []struct {
		label string
		addr  uint32
		kind  string // "u32", "s16", "string"
	}{
		{"global_variant_at_2f90a8_u32", 0x2F90A8, "u32"},
		{"game_variant_global_at_2fab60_u32", 0x2FAB60, "u32"},
		{"game_connection_at_2e3684_s16", 0x2E3684, "s16"},
		{"global_stage_at_2fac20_str", 0x2FAC20, "string"},
		{"multiplayer_map_name_at_2e37cd_str", 0x2E37CD, "string"},
	} {
		hva, err := r.inst.LowHVA(c.addr)
		if err != nil {
			continue
		}
		switch c.kind {
		case "u32":
			if v, err := r.inst.Mem.ReadU32At(hva); err == nil {
				out[c.label] = v
			}
		case "s16":
			if v, err := r.inst.Mem.ReadS16At(hva); err == nil {
				out[c.label] = v
			}
		case "string":
			if b, err := r.inst.Mem.ReadBytesAt(hva, 32); err == nil {
				out[c.label] = trimNul(string(b))
			}
		}
	}
	return out
}

func (r *Reader) probeTeamScoresRaw() map[string]any {
	out := map[string]any{}
	bases := []struct {
		label string
		addr  uint32
		count int
	}{
		{"ctf_at_2762b4_u32_x2", AddrScoreCTF, 2},
		{"slayer_at_276710_u32_x16", AddrScoreSlayer, 16},
		{"oddball_at_27653c_u32_x2", AddrScoreOddball, 2},
		{"king_at_2762d8_u32_x2", AddrScoreKing, 2},
		{"race_at_2766c8_u32_x2", AddrScoreRace, 2},
	}
	for _, b := range bases {
		hva, err := r.inst.LowHVA(b.addr)
		if err != nil {
			continue
		}
		vs := make([]uint32, 0, b.count)
		for i := 0; i < b.count; i++ {
			v, err := r.inst.Mem.ReadU32At(hva + int64(i*4))
			if err != nil {
				break
			}
			vs = append(vs, v)
		}
		out[b.label] = vs
	}
	return out
}

func (r *Reader) probeScoreLimitsRaw() map[string]any {
	out := map[string]any{}
	limits := []struct {
		label string
		addr  uint32
	}{
		{"ctf_limit_at_2762bc_u32", AddrScoreLimitCTF},
		{"slayer_limit_at_2f90e8_u32", AddrScoreLimitSlayer},
		{"oddball_limit_at_276538_u32", AddrScoreLimitOddball},
	}
	for _, lm := range limits {
		hva, err := r.inst.LowHVA(lm.addr)
		if err != nil {
			continue
		}
		v, err := r.inst.Mem.ReadU32At(hva)
		if err != nil {
			continue
		}
		out[lm.label] = v
	}
	return out
}

func (r *Reader) probePerPlayerScoreTables() map[string]any {
	out := map[string]any{}
	bases := []struct {
		label string
		addr  uint32
	}{
		{"slayer_table_at_276710_plus_64_s32_x16", AddrScoreSlayer},
		{"oddball_table_at_27653c_plus_64_s32_x16", AddrScoreOddball},
		{"king_table_at_2762d8_plus_64_s32_x16", AddrScoreKing},
		{"race_table_at_2766c8_plus_64_s32_x16", AddrScoreRace},
	}
	for _, b := range bases {
		hva, err := r.inst.LowHVA(b.addr)
		if err != nil {
			continue
		}
		tableHVA := hva + int64(PlayerScoreBaseOffset)
		vs := make([]int32, 0, 16)
		for i := 0; i < 16; i++ {
			v, err := r.inst.Mem.ReadU32At(tableHVA + int64(i*4))
			if err != nil {
				break
			}
			vs = append(vs, int32(v))
		}
		out[b.label] = vs
	}
	return out
}

// probePerPlayerStaticStruct walks the PlayerDatumArray and dumps the
// per-player ctf_score field at OffPlrCTFScore (0xC4). Empirically this
// holds the slayer score in slayer matches as well, so it's the most
// gametype-agnostic per-player score we have today.
func (r *Reader) probePerPlayerStaticStruct() any {
	pdaBase, err := r.inst.DerefLowPtr(AddrPlayerDatumArrayPtr)
	if err != nil || pdaBase < HighGVAThreshold {
		return nil
	}
	elemSize, err := r.inst.Mem.ReadU16(pdaBase + OffPDAElementSize)
	if err != nil || elemSize == 0 {
		return nil
	}
	currentCount, _ := r.inst.Mem.ReadU16(pdaBase + OffPDACurrentCount)
	firstElement, err := r.inst.Mem.ReadU32(pdaBase + OffPDAFirstElement)
	if err != nil || firstElement < HighGVAThreshold {
		return nil
	}

	type entry struct {
		Index             int    `json:"index"`
		Name              string `json:"name"`
		CTFScoreOffsetC4  int16  `json:"ctf_score_offset_c4_s16"`
	}
	entries := make([]entry, 0, currentCount)
	for i := uint16(0); i < currentCount; i++ {
		base := firstElement + uint32(i)*uint32(elemSize)
		nameBytes, err := r.inst.Mem.ReadBytes(base+OffPlrName, 24)
		if err != nil {
			continue
		}
		if nameBytes[0] == 0 && nameBytes[1] == 0 {
			continue
		}
		ctf, _ := r.inst.Mem.ReadS16(base + OffPlrCTFScore)
		entries = append(entries, entry{
			Index:            int(i),
			Name:             decodeUTF16LE(nameBytes),
			CTFScoreOffsetC4: ctf,
		})
	}
	return entries
}

func trimNul(s string) string {
	for i, c := range s {
		if c == 0 {
			return s[:i]
		}
	}
	return s
}
