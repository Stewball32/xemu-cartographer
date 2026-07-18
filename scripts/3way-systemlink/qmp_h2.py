#!/usr/bin/env python3
"""qmp_h2.py — probe Halo 2 live state from an xemu QMP socket via memsave.

Reuses the proven qmp_mem.QMP (HMP memsave) class. Resolves the W1-W4 datum-array
offset map (halo-offset-mapper/offset-maps/h2-stock.offsets.json) live:
  static .data ptr -> s_data_array header (+0x44 block, +0x38 active, +0x2C sig)
  players[16] (stride 0x21C): index@0x1C team@0x20 unit@0x2C name@0x44(wchar16)
  unit handle -> object entry table (obj_array+0x44, stride 0xC, data_ptr@0x8)
  biped: health@0x84 shield@0xA0 pos@0x34 maxhp@0xE4 maxsh@0xE8 frags@0x23E ...
Usage:  qmp_h2.py SOCK [--json]
"""
import sys, os, json, struct, time
HERE = os.path.dirname(os.path.abspath(__file__))
sys.path.insert(0, HERE)
import qmp_mem  # QMP class + utf16 + cstr

# ---- offsets (from h2-stock.offsets.json, all runtime-verified unless noted) --
A_PLAYERS = 0x4F01F4
A_OBJECTS = 0x4E78D0
A_TAGHDR  = 0x54F24C
A_SCNPOOL = 0x4EDA3C
# data-array header
DA_MAX, DA_ELEM, DA_SIG, DA_ACTIVE, DA_BLK, DA_BLKEND = 0x20, 0x24, 0x2C, 0x38, 0x44, 0x48
# object entry
OE_SALT, OE_DATA, OE_STRIDE = 0x0, 0x8, 0xC
# player datum
P_STRIDE = 0x21C
P_DATUMID, P_PLAYERID, P_INDEX, P_TEAM, P_UNIT, P_NAME = 0x0, 0x4, 0x1C, 0x20, 0x2C, 0x44
# object/biped data
O_TAGID, O_HELDWEP, O_POS, O_FWD, O_HEALTH, O_VEL, O_SHIELD = 0x0, 0x10, 0x34, 0x70, 0x84, 0x88, 0xA0
O_TYPE, O_MAXHP, O_MAXSH, O_AIM, O_CURWEP, O_FRAG, O_PLASMA = 0xAA, 0xE4, 0xE8, 0x150, 0x212, 0x23E, 0x23F
# weapon
W_RESERVE, W_MAG = 0x22A, 0x22C
# tag header
TH_INST, TH_SCENID, TH_GLOBID, TH_COUNT = 0x8, 0xC, 0x10, 0x18
TI_GROUP, TI_ID, TI_DATA, TI_SIZE, TI_STRIDE = 0x0, 0x4, 0x8, 0xC, 0x10

def f32(q, a):
    d = q.memsave(a, 4)
    return struct.unpack("<f", d)[0] if len(d) == 4 else None

def vec(q, a, n):
    d = q.memsave(a, 4*n)
    return list(struct.unpack("<%df" % n, d)) if len(d) == 4*n else None

def wname(q, a, chars=16):
    return qmp_mem.utf16(q.memsave(a, chars*2))

def valid_ptr(p):
    return p is not None and 0x10000 <= p < 0xFFFFFFFF

def read_array_header(q, static_ptr):
    """deref static .data ptr -> data_array; read header fields."""
    base = q.u32(static_ptr)
    h = {"static_ptr": hex(static_ptr), "array_ptr": hex(base) if base else None}
    if not valid_ptr(base):
        h["ok"] = False; return h, base
    sig = q.memsave(base + DA_SIG, 4)
    h["signature"] = sig.decode("latin1")
    h["signature_hex"] = sig.hex()
    h["max"] = q.u32(base + DA_MAX)
    h["elem_size"] = q.u32(base + DA_ELEM)
    h["active"] = q.u32(base + DA_ACTIVE)
    h["block_ptr"] = hex(q.u32(base + DA_BLK) or 0)
    h["block_end"] = hex(q.u32(base + DA_BLKEND) or 0)
    h["ok"] = valid_ptr(q.u32(base + DA_BLK))
    return h, base

def resolve_unit(q, obj_array_base, handle):
    """unit handle -> object entry table -> object data ptr."""
    if not handle or handle == 0xFFFFFFFF:
        return None, None
    blk = q.u32(obj_array_base + DA_BLK)
    if not valid_ptr(blk):
        return None, None
    idx = handle & 0xFFFF
    entry = blk + idx * OE_STRIDE
    salt = q.u16(entry + OE_SALT)
    data = q.u32(entry + OE_DATA)
    return (data if valid_ptr(data) else None), {"idx": idx, "salt": salt, "entry": hex(entry)}

def probe(sock):
    q = qmp_mem.QMP(sock)
    out = {"sock": sock, "ts": round(time.time(), 2)}
    # --- title id (XBE) ---
    xbe = 0x00010000
    magic = q.u32(xbe)
    out["xbe_magic"] = hex(magic or 0)
    out["xbe_magic_ascii"] = struct.pack("<I", magic).decode("latin1") if magic else ""
    cert = q.u32(xbe + 0x0118)
    tid = q.u32(cert + 0x08) if valid_ptr(cert) else None
    out["cert_ptr"] = hex(cert or 0)
    out["title_id"] = hex(tid or 0)
    out["title_id_is_h2"] = (tid == 0x4D530064)
    # --- arrays ---
    ph, pbase = read_array_header(q, A_PLAYERS)
    oh, obase = read_array_header(q, A_OBJECTS)
    out["players_array"] = ph
    out["objects_array"] = oh
    # --- roster + bipeds ---
    roster = []
    if ph.get("ok"):
        blk = q.u32(pbase + DA_BLK)
        pmax = min(ph.get("max") or 16, 16)
        for i in range(pmax):
            rec = blk + i * P_STRIDE
            did = q.u32(rec + P_DATUMID)
            if not did or did == 0xFFFFFFFF:
                continue
            name = wname(q, rec + P_NAME)
            unit = q.u32(rec + P_UNIT)
            e = {"slot": i, "datum_id": hex(did), "index": q.s32(rec + P_INDEX) if hasattr(q,'s32') else struct.unpack('<i',q.memsave(rec+P_INDEX,4))[0],
                 "team": struct.unpack('<i', q.memsave(rec + P_TEAM, 4))[0],
                 "name": name, "unit_handle": hex(unit or 0)}
            if obase and valid_ptr(unit):
                odata, meta = resolve_unit(q, obase, unit)
                e["obj_meta"] = meta
                if odata:
                    e["obj_data"] = hex(odata)
                    e["health"] = f32(q, odata + O_HEALTH)
                    e["shield"] = f32(q, odata + O_SHIELD)
                    e["max_health"] = f32(q, odata + O_MAXHP)
                    e["max_shield"] = f32(q, odata + O_MAXSH)
                    e["position"] = vec(q, odata + O_POS, 3)
                    e["forward"] = vec(q, odata + O_FWD, 2)
                    e["frags"] = q.u8(odata + O_FRAG)
                    e["plasmas"] = q.u8(odata + O_PLASMA)
                    e["tag_id"] = hex(q.u32(odata + O_TAGID) or 0)
                    hw = q.u32(odata + O_HELDWEP)
                    e["held_weapon_handle"] = hex(hw or 0)
                    if obase and valid_ptr(hw):
                        wdata, _ = resolve_unit(q, obase, hw)
                        if wdata:
                            e["weapon_mag"] = q.u16(wdata + W_MAG)
                            e["weapon_reserve"] = q.u16(wdata + W_RESERVE)
            roster.append(e)
    out["roster"] = roster
    # --- map via tag header ---
    try:
        th = q.u32(A_TAGHDR)
        m = {"tag_header": hex(th or 0)}
        if valid_ptr(th):
            m["scenario_id"] = hex(q.u32(th + TH_SCENID) or 0)
            m["globals_id"]  = hex(q.u32(th + TH_GLOBID) or 0)
            m["tag_count"]   = q.u32(th + TH_COUNT)
            inst = q.u32(th + TH_INST)
            scen = q.u32(th + TH_SCENID) or 0
            if valid_ptr(inst):
                ti = inst + (scen & 0xFFFF) * TI_STRIDE
                grp = q.memsave(ti + TI_GROUP, 4)
                m["scenario_group"] = grp[::-1].decode("latin1", "ignore")
                m["scenario_data"] = hex(q.u32(ti + TI_DATA) or 0)
        out["map"] = m
    except Exception as ex:
        out["map"] = {"err": repr(ex)}
    q.close()
    return out

if __name__ == "__main__":
    if len(sys.argv) < 2:
        sys.exit("usage: qmp_h2.py SOCK [--json]")
    r = probe(sys.argv[1])
    print(json.dumps(r, indent=2))
