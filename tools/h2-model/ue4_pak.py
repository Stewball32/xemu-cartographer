#!/usr/bin/env python3
"""Minimal, dependency-free UE4 .pak reader (FPakInfo version 7).

MCC ships its customization UI (incl. the Spartan customization *pose* assets) in
`MCC/Content/Paks/MCC-WindowsNoEditor.pak` — a UE4 pak whose index is
**unencrypted**, so we can list + extract individual `.uasset/.uexp/.ubulk`
chunks in pure Python. This is the accessible, cross-game source for MCC's 11
customization stances (Default, At Ease, Salute, Crossed Arms, Hands on Hips,
Double Flex, Kneel, Look Back, Peace Sign, Meditation, Yoga), which we then
retarget onto the H2 Mark VI rig.

Format authority: Epic's FPakFile/FPakInfo/FPakEntry serialization (UE 4.x).
IP: personal / LAN use with your own copy of MCC. Do not redistribute assets.
"""
import struct, sys, os, zlib, argparse

MAGIC = 0x5A6F12E1


class R:
    def __init__(self, b, o=0):
        self.b = b; self.o = o
    def i32(self):  v = struct.unpack_from('<i', self.b, self.o)[0]; self.o += 4; return v
    def u32(self):  v = struct.unpack_from('<I', self.b, self.o)[0]; self.o += 4; return v
    def i64(self):  v = struct.unpack_from('<q', self.b, self.o)[0]; self.o += 8; return v
    def u8(self):   v = self.b[self.o]; self.o += 1; return v
    def raw(self, n): v = self.b[self.o:self.o + n]; self.o += n; return v
    def fstring(self):
        n = self.i32()
        if n == 0:
            return ""
        if n < 0:
            n = -n
            s = self.b[self.o:self.o + n * 2].decode('utf-16-le', 'replace')
            self.o += n * 2
        else:
            s = self.b[self.o:self.o + n].decode('latin1', 'replace')
            self.o += n
        return s.split('\x00', 1)[0]


def read_footer(f, size):
    FOOT = 16 + 1 + 4 + 4 + 8 + 8 + 20
    f.seek(size - FOOT); blob = f.read(FOOT)
    guid = blob[0:16]; enc = blob[16]
    magic, ver, idx_off, idx_size = struct.unpack_from('<IiqQ', blob, 17)
    assert magic == MAGIC, f"bad pak magic {magic:#x}"
    return dict(enc=enc, ver=ver, idx_off=idx_off, idx_size=idx_size, guid=guid)


def parse_entry(r, ver):
    """FPakEntry (v7 layout): returns dict + advances reader past the record."""
    off = r.i64(); size = r.i64(); usize = r.i64(); cmethod = r.i32()
    h = r.raw(20)
    blocks = []
    if cmethod != 0:
        n = r.i32()
        for _ in range(n):
            blocks.append((r.i64(), r.i64()))
    flags = r.u8(); cblock = r.u32()
    return dict(off=off, size=size, usize=usize, cmethod=cmethod,
                blocks=blocks, flags=flags, cblock=cblock)


def entry_header_len(e):
    """Size of the FPakEntry header as re-serialized in front of the payload."""
    n = 8 + 8 + 8 + 4 + 20
    if e['cmethod'] != 0:
        n += 4 + len(e['blocks']) * 16
    n += 1 + 4
    return n


class Pak:
    def __init__(self, path):
        self.path = path
        self.f = open(path, 'rb')
        self.f.seek(0, 2); self.size = self.f.tell()
        self.info = read_footer(self.f, self.size)
        assert self.info['enc'] == 0, "index is encrypted (need AES key)"
        self.f.seek(self.info['idx_off'])
        idx = self.f.read(self.info['idx_size'])
        r = R(idx)
        self.mount = r.fstring()
        self.count = r.i32()
        self.entries = {}
        for _ in range(self.count):
            name = r.fstring()
            e = parse_entry(r, self.info['ver'])
            self.entries[name] = e
        self.trailing = len(idx) - r.o  # 0 if perfectly aligned

    def read(self, name):
        """Return the decompressed payload bytes for an index entry."""
        e = self.entries[name]
        hdr = entry_header_len(e)
        data_off = e['off'] + hdr
        if e['cmethod'] == 0:
            self.f.seek(data_off); return self.f.read(e['size'])
        out = bytearray()
        for (cs, ce) in e['blocks']:
            # v5+ : block offsets relative to entry start (e['off'])
            start = e['off'] + cs if cs < e['off'] else cs
            self.f.seek(start); comp = self.f.read(ce - cs)
            out += zlib.decompress(comp)
        return bytes(out)


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("pak")
    ap.add_argument("--grep", default=None, help="case-insensitive substring filter")
    ap.add_argument("--extract", default=None, help="exact entry name to extract")
    ap.add_argument("--out", default=None)
    ap.add_argument("--limit", type=int, default=80)
    a = ap.parse_args()
    pk = Pak(a.pak)
    print(f"mount={pk.mount!r} entries={pk.count} ver={pk.info['ver']} "
          f"trailing_index_bytes={pk.trailing}")
    if a.extract:
        data = pk.read(a.extract)
        out = a.out or os.path.basename(a.extract)
        open(out, 'wb').write(data)
        print(f"wrote {out} ({len(data)} bytes)")
        return
    if a.grep:
        g = a.grep.lower()
        hits = [n for n in pk.entries if g in n.lower()]
        print(f"{len(hits)} match '{a.grep}':")
        for n in sorted(hits)[:a.limit]:
            e = pk.entries[n]
            print(f"  {n}  [off={e['off']} size={e['size']} usize={e['usize']} cm={e['cmethod']}]")


if __name__ == "__main__":
    main()
