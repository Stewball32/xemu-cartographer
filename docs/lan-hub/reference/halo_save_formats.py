#!/usr/bin/env python3
"""
halo_save_formats.py  —  Reverse-engineered readers/writers for Halo CE & Halo 2
Xbox UDATA save files (player profiles + gametype/variant files), for the
xemu-cartographer LAN hub.

Derived offline (2026-06-23/24) by diffing real samples extracted READ-ONLY from
the fleet xemu HDD images (pyfatx). No xemu was launched.

Covered file types
------------------
  CE gametype  : E:\\UDATA\\4d530004\\<name>\\blam.lst         (fixed 512 bytes)
  CE save meta : E:\\UDATA\\4d530004\\<name>\\SaveMeta.xbx     (UTF-16 "Name=..")
  H2 profile   : E:\\UDATA\\4d530064\\<id>\\profile            (500 bytes here)
  H2 gametype  : E:\\UDATA\\4d530064\\<id>\\<modename>         (e.g. 'slayer', 324B)
  SaveMeta.xbx : both titles, identical trivial format

Design philosophy: TEMPLATE-PATCH
---------------------------------
Every save carries a 20-byte content-dependent integrity DIGEST whose algorithm
is not yet cracked (see DIGEST_NOTE). So the generator never builds a file from
nothing: it takes a real same-engine sample as a TEMPLATE, overwrites only the
fields we understand, and preserves every other byte (including the digest and
the trailing padding) verbatim. Consequences:
  * build(template, <no changes>)            -> byte-identical to template.
  * build(template, score_limit=100, ...)    -> identical except the patched
                                                field bytes (digest NOT updated).
`recompute_digest()` is a pluggable hook (currently a no-op that preserves the
template bytes). Drop the real algorithm in there once the runtime probe
(see DE-RISK-VERDICT.md) tells us whether the digest is even enforced.
"""
from __future__ import annotations
import struct
from dataclasses import dataclass, field
from typing import Optional

# ---------------------------------------------------------------------------
DIGEST_NOTE = (
    "20-byte content-dependent digest. NOT a plain SHA-1/MD5/CRC of any "
    "contiguous range, NOT HMAC-SHA1 under obvious keys (tested exhaustively). "
    "Most likely the Xbox content signature (HMAC-SHA1 under a per-title key) "
    "or a Halo-internal salted hash. Preserved from template until cracked."
)

U32 = struct.Struct("<I")
def u32(b, o): return U32.unpack_from(b, o)[0]
def pu32(ba, o, v): U32.pack_into(ba, o, v & 0xFFFFFFFF)

def _utf16z_read(b: bytes, off: int, maxbytes: int) -> str:
    raw = b[off:off+maxbytes]
    i = 0
    out = []
    while i + 1 < len(raw):
        ch = raw[i] | (raw[i+1] << 8)
        if ch == 0:
            break
        out.append(chr(ch))
        i += 2
    return "".join(out)

def _utf16z_write(ba: bytearray, off: int, bufbytes: int, name: str):
    """Write name as UTF-16LE NUL-terminated, zero-filling the whole buffer."""
    enc = name.encode("utf-16-le")
    if len(enc) + 2 > bufbytes:
        raise ValueError(f"name {name!r} too long for {bufbytes}-byte buffer "
                         f"(max {(bufbytes-2)//2} chars)")
    ba[off:off+bufbytes] = b"\x00" * bufbytes
    ba[off:off+len(enc)] = enc  # trailing NUL(s) already zero

# ===========================================================================
# SaveMeta.xbx  —  display-name sidecar (UNSIGNED, trivially generatable)
#   bytes: FF FE  +  "Name=<display>\r\n" in UTF-16LE
# ===========================================================================
def savemeta_parse(b: bytes) -> str:
    assert b[:2] == b"\xff\xfe", "missing UTF-16LE BOM"
    text = b[2:].decode("utf-16-le")
    assert text.startswith("Name="), f"unexpected SaveMeta body {text!r}"
    return text[len("Name="):].rstrip("\r\n")

def savemeta_build(display_name: str) -> bytes:
    return b"\xff\xfe" + (f"Name={display_name}\r\n").encode("utf-16-le")

# ===========================================================================
# Halo CE gametype  —  blam.lst, fixed 512 bytes
# ===========================================================================
CE_ENGINE = {1: "ctf", 2: "slayer", 3: "oddball", 4: "king", 5: "race"}
CE_ENGINE_INV = {v: k for k, v in CE_ENGINE.items()}

# Field offsets within blam.lst (verified by diffing 23 real variants)
CE_OFF = dict(
    name=0x00, name_buf=0x18,          # UTF-16LE name, 24-byte buffer
    engine=0x18, teams=0x1C, options=0x20, scoring_subtype=0x24,
    time_limit=0x30, time_limit2=0x34, speed=0x3C,
    score_limit=0x40, option2=0x44, respawn=0x48, engine_union=0x4C,
    digest=0x68, digest_len=20,        # 0x68..0x7B
    tail=0x7C,                         # 0x7C..0x1FF preserved padding
    size=0x200,
)

@dataclass
class CEGametype:
    raw: bytes
    name: str = ""
    engine: int = 0
    engine_name: str = ""
    teams: int = 0
    options: int = 0
    scoring_subtype: int = 0
    time_limit: int = 0
    time_limit2: int = 0
    score_limit: int = 0
    option2: int = 0
    respawn: int = 0
    engine_union: int = 0
    digest: bytes = b""

def ce_parse(b: bytes) -> CEGametype:
    assert len(b) == CE_OFF["size"], f"CE blam.lst must be 512B, got {len(b)}"
    g = CEGametype(raw=b)
    g.name = _utf16z_read(b, CE_OFF["name"], CE_OFF["name_buf"])
    g.engine = u32(b, CE_OFF["engine"]);  g.engine_name = CE_ENGINE.get(g.engine, "?")
    g.teams = u32(b, CE_OFF["teams"])
    g.options = u32(b, CE_OFF["options"])
    g.scoring_subtype = u32(b, CE_OFF["scoring_subtype"])
    g.time_limit = u32(b, CE_OFF["time_limit"])
    g.time_limit2 = u32(b, CE_OFF["time_limit2"])
    g.score_limit = u32(b, CE_OFF["score_limit"])
    g.option2 = u32(b, CE_OFF["option2"])
    g.respawn = u32(b, CE_OFF["respawn"])
    g.engine_union = u32(b, CE_OFF["engine_union"])
    g.digest = b[CE_OFF["digest"]:CE_OFF["digest"]+CE_OFF["digest_len"]]
    return g

def ce_build(template: bytes, *, name=None, engine=None, teams=None, options=None,
             scoring_subtype=None, time_limit=None, score_limit=None, option2=None,
             respawn=None, engine_union=None, recompute=False) -> bytes:
    """Patch a real CE blam.lst template. engine may be int or name string."""
    ba = bytearray(template)
    assert len(ba) == CE_OFF["size"]
    if name is not None:
        _utf16z_write(ba, CE_OFF["name"], CE_OFF["name_buf"], name)
    if engine is not None:
        engine = CE_ENGINE_INV.get(engine, engine) if isinstance(engine, str) else engine
        pu32(ba, CE_OFF["engine"], engine)
    if teams is not None:           pu32(ba, CE_OFF["teams"], teams)
    if options is not None:         pu32(ba, CE_OFF["options"], options)
    if scoring_subtype is not None: pu32(ba, CE_OFF["scoring_subtype"], scoring_subtype)
    if time_limit is not None:      pu32(ba, CE_OFF["time_limit"], time_limit)
    if score_limit is not None:     pu32(ba, CE_OFF["score_limit"], score_limit)
    if option2 is not None:         pu32(ba, CE_OFF["option2"], option2)
    if respawn is not None:         pu32(ba, CE_OFF["respawn"], respawn)
    if engine_union is not None:    pu32(ba, CE_OFF["engine_union"], engine_union)
    out = bytes(ba)
    if recompute:
        out = recompute_digest(out, CE_OFF["digest"], CE_OFF["digest_len"], title="ce")
    return out

def ce_minutes_to_raw(minutes: float) -> int:
    """Observed: raw 300 = '10 min' label, 225 = '7.5', 150 = '5'. unit = 2s."""
    return int(round(minutes * 30))
def ce_raw_to_minutes(raw: int) -> float:
    return raw / 30.0

# ===========================================================================
# Halo 2 gametype  —  variable size (slayer sample = 324 bytes)
#   0x00 u32 version (00 00 00 01)
#   0x04 UTF-16LE name (NUL-term)
#   ...settings... (0x50 = score limit in slayer sample)
#   trailing 20 bytes = digest
#   NOTE: only one H2 gametype sample available -> field map is partial.
# ===========================================================================
H2GT_OFF = dict(version=0x00, name=0x04, name_buf=0x40,
                score_limit=0x50, digest_len=20)

@dataclass
class H2Gametype:
    raw: bytes
    version: int = 0
    name: str = ""
    score_limit: int = 0
    digest: bytes = b""

def h2gt_parse(b: bytes) -> H2Gametype:
    g = H2Gametype(raw=b)
    g.version = u32(b, H2GT_OFF["version"])
    g.name = _utf16z_read(b, H2GT_OFF["name"], H2GT_OFF["name_buf"])
    g.score_limit = u32(b, H2GT_OFF["score_limit"])
    g.digest = b[-H2GT_OFF["digest_len"]:]
    return g

def h2gt_build(template: bytes, *, name=None, score_limit=None, recompute=False) -> bytes:
    ba = bytearray(template)
    if name is not None:
        _utf16z_write(ba, H2GT_OFF["name"], H2GT_OFF["name_buf"], name)
    if score_limit is not None:
        pu32(ba, H2GT_OFF["score_limit"], score_limit)
    out = bytes(ba)
    if recompute:
        out = recompute_digest(out, len(out)-H2GT_OFF["digest_len"],
                               H2GT_OFF["digest_len"], title="h2")
    return out

# ===========================================================================
# Halo 2 player profile  —  500 bytes
#   0x00..0x07 header (zero)
#   0x08 UTF-16LE player name (NUL-term, large buffer)
#   0x118.. appearance + controller block (per-profile bytes)
#   trailing 20 bytes (0x1E0..0x1F3) = digest
#   Appearance/control byte labels are PROVISIONAL (2 samples). The block is
#   exposed raw so it can be copied from a template or patched byte-wise.
# ===========================================================================
H2P_OFF = dict(name=0x08, name_buf=0x110,
               appctl=0x118, appctl_len=0x18,   # 0x118..0x12F observed-variant block
               digest=0x1E0, digest_len=20, size=0x1F4)
# provisional single-byte labels within the appearance/control block:
H2P_APPCTL_FIELDS = {
    0x118: "armor_primary?",  0x119: "armor_secondary?",
    0x11A: "emblem_fg?",      0x11B: "emblem_bg?",
    0x11D: "emblem_primary_color?", 0x11E: "emblem_secondary_color?",
    0x12B: "ctrl_a?", 0x12F: "ctrl_b?",
}

@dataclass
class H2Profile:
    raw: bytes
    name: str = ""
    appctl: bytes = b""
    appctl_fields: dict = field(default_factory=dict)
    digest: bytes = b""

def h2p_parse(b: bytes) -> H2Profile:
    assert len(b) == H2P_OFF["size"], f"H2 profile must be 500B, got {len(b)}"
    p = H2Profile(raw=b)
    p.name = _utf16z_read(b, H2P_OFF["name"], H2P_OFF["name_buf"])
    a = H2P_OFF["appctl"]
    p.appctl = b[a:a+H2P_OFF["appctl_len"]]
    p.appctl_fields = {f"{off:#05x}:{lab}": b[off] for off, lab in H2P_APPCTL_FIELDS.items()}
    p.digest = b[H2P_OFF["digest"]:H2P_OFF["digest"]+H2P_OFF["digest_len"]]
    return p

def h2p_build(template: bytes, *, name=None, appctl: Optional[bytes]=None,
              appctl_patch: Optional[dict]=None, recompute=False) -> bytes:
    """name: new player name. appctl: replace whole 0x118-block. appctl_patch:
    {offset:byte} single-byte patches (e.g. {0x118: 7} to set armor colour)."""
    ba = bytearray(template)
    assert len(ba) == H2P_OFF["size"]
    if name is not None:
        _utf16z_write(ba, H2P_OFF["name"], H2P_OFF["name_buf"], name)
    if appctl is not None:
        a = H2P_OFF["appctl"]
        assert len(appctl) == H2P_OFF["appctl_len"]
        ba[a:a+H2P_OFF["appctl_len"]] = appctl
    if appctl_patch:
        for off, val in appctl_patch.items():
            ba[off] = val & 0xFF
    out = bytes(ba)
    if recompute:
        out = recompute_digest(out, H2P_OFF["digest"], H2P_OFF["digest_len"], title="h2")
    return out

# ===========================================================================
# Digest hook  —  CRACKED 2026-06-24.  Original-Xbox roamable save signature.
#   signature(20B) = HMAC-SHA1(AuthKey, data) ; data = buf[:sig_off]
#   AuthKey(16B)   = HMAC-SHA1(XBOX_GLOBAL_KEY, cert.sig_key)[:16]
# Verified byte-exact on every real sample (CE 23/23, H2 3/3) and on fresh edits.
# Full write-up + provenance: SIGNATURE-CRACK.md.  Standalone signer: xbox_save_sig.py.
# Roamable => per-title key only (portable across consoles; no XboxHDKey needed).
# ===========================================================================
import hashlib, hmac

# Global "Xbox key" baked into the Xbox OS, public (feudalnate XSavSig / gothi.co.uk).
XBOX_GLOBAL_KEY = bytes.fromhex("5C0733AE0401F7E8BA7993FDCD2F1FE0")

# Per-title 16-byte certificate signature keys (plaintext @cert+0xC0 in retail XBEs).
# These equal the keys extracted live from Halo CE/H2 default.xbe (cross-checked).
TITLE_SIG_KEYS = {
    "ce": bytes.fromhex("1F71DE93D52AADB19446D7494F731158"),  # Halo CE  title 0x4D530004
    "h2": bytes.fromhex("2116D927510F01D19B7EC75CAFE669AC"),  # Halo 2   title 0x4D530064
}

def derive_auth_key(sig_key: bytes) -> bytes:
    """Per-title signing key = HMAC-SHA1(XboxKey, cert.sig_key) truncated to 16 bytes."""
    return hmac.new(XBOX_GLOBAL_KEY, sig_key, hashlib.sha1).digest()[:16]

def xbe_sig_key(xbe_path: str) -> bytes:
    """Read the 16-byte certificate signature key straight from a retail XBE (cert+0xC0)."""
    d = open(xbe_path, "rb").read()
    if d[:4] != b"XBEH":
        raise ValueError("not an XBE (bad magic)")
    base = struct.unpack_from("<I", d, 0x104)[0]
    cert = struct.unpack_from("<I", d, 0x118)[0] - base
    return d[cert + 0xC0: cert + 0xD0]

def auth_key_for(title: str, sig_key: Optional[bytes] = None) -> bytes:
    """AuthKey for a title. Pass sig_key (16B, e.g. from xbe_sig_key()) to override
    the built-in table — lets you sign for any title/region from its XBE."""
    if sig_key is None:
        if title not in TITLE_SIG_KEYS:
            raise ValueError(f"unknown title {title!r}; known {list(TITLE_SIG_KEYS)} "
                             f"or pass sig_key=xbe_sig_key('default.xbe')")
        sig_key = TITLE_SIG_KEYS[title]
    return derive_auth_key(sig_key)

def recompute_digest(buf: bytes, off: int, length: int, title: str,
                     sig_key: Optional[bytes] = None) -> bytes:
    """Write a correct Xbox roamable save signature into buf[off:off+length].
    The signed message is everything before the signature field (buf[:off]); this is
    correct for CE (sig embedded at 0x68 over the 104 preceding bytes) and for H2
    (sig trailing over all preceding bytes). Bytes after the signature (CE padding)
    are not signed and are preserved as-is."""
    auth = auth_key_for(title, sig_key)
    sig = hmac.new(auth, buf[:off], hashlib.sha1).digest()[:length]
    ba = bytearray(buf)
    ba[off:off + length] = sig
    return bytes(ba)
