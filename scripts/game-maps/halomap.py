"""
Halo: Combat Evolved (Xbox) map-cache parser — static, build-agnostic.

Parses the .map cache-file header and tag index, transparently handling the
zlib-compressed body used by the modded Xbox builds (and falling back to an
uncompressed body for stock/unpacked maps). Everything is derived from each
map's *actual* data — no hardcoded tag counts, addresses, or map sets.

Format (Xbox Halo 1 cache):
  0x000  uint32  head literal            0x68656164 ('head', LE 'daeh')
  0x004  uint32  engine/version          5 = Xbox
  0x008  uint32  decompressed_size       full image incl. 2048-byte header
  0x00C  uint32  (padding / compressed)
  0x010  uint32  tag_data_offset         offset into the decompressed image
  0x014  uint32  tag_data_size
  0x020  char32  map name                e.g. "bloodgulch"
  0x040  char32  build string            e.g. "01.10.12.2276"
  0x060  uint32  map type                0=campaign(sp) 1=multiplayer 2=ui/mainmenu
  0x064  uint32  checksum / map id
  0x7FC  uint32  foot literal            0x666F6F74 ('foot', LE 'toof')

If bytes at 0x800 are a zlib stream (0x78 ..), the body is inflated and
prepended with the raw 2048-byte header to reconstruct the in-memory image,
so that tag_data_offset / virtual pointers resolve directly.

Tag index header (at tag_data_offset), 0x24 bytes:
  +0x00 uint32  tag_array_vaddr     virtual address of first tag element
  +0x04 uint32  scenario_tag_id     datum
  +0x08 uint32  map_id / checksum
  +0x0C uint32  tag_count
  +0x10 uint32  model_part_count
  +0x14 uint32  model_data_vaddr
  +0x18 uint32  model_part_count2
  +0x1C uint32  vertex_data_vaddr
  +0x20 uint32  'tags' literal      0x73676174
Tag array follows at +0x24. Each tag is 0x20 bytes:
  +0x00 uint32  primary group fourcc   (e.g. 'scnr','bitm','unic','matg')
  +0x04 uint32  secondary group        (0xFFFFFFFF if none)
  +0x08 uint32  tertiary group
  +0x0C uint32  tag_id (datum)
  +0x10 uint32  path_vaddr             -> null-terminated ASCII path
  +0x14 uint32  meta_vaddr             -> tag data (meta) in the image
  +0x18 uint32  indexed flag           (1 = data external/indexed on Xbox)
  +0x1C uint32  (zero)

The virtual base is derived from the data itself (tag_array_vaddr - 0x24),
so no per-build base constant is hardcoded.
"""

from __future__ import annotations

import struct
import zlib
from dataclasses import dataclass, field
from typing import Optional

HEAD_LITERAL = 0x68656164
FOOT_LITERAL = 0x666F6F74
TAGS_LITERAL = 0x74616773  # 'tags' as stored (LE read of file bytes 73 67 61 74)
HEADER_SIZE = 0x800
TAG_ENTRY_SIZE = 0x20
TAG_INDEX_HEADER_SIZE = 0x24

MAP_TYPE_NAMES = {0: "campaign", 1: "multiplayer", 2: "ui"}


def fourcc(v: int) -> str:
    """Render a 32-bit tag-group literal as its 4-char string (big-endian read)."""
    b = struct.pack(">I", v & 0xFFFFFFFF)
    return "".join(chr(c) if 32 <= c < 127 else "." for c in b)


class MapParseError(Exception):
    pass


@dataclass
class Tag:
    index: int
    group: str            # primary group fourcc, e.g. "bitm"
    group_raw: int
    tag_id: int
    path: str             # tag path, e.g. "ui\\shell\\..." (no extension)
    path_vaddr: int
    meta_vaddr: int
    indexed: int


@dataclass
class HaloMap:
    path: str
    name: str
    build: str
    version: int
    map_type: int
    decompressed_size: int
    tag_data_offset: int
    tag_data_size: int
    compressed: bool
    image: bytes = field(repr=False)          # full reconstructed in-memory image
    vbase: int = 0                            # virtual base address of tag space
    scenario_tag_id: int = 0
    map_id: int = 0
    tags: list[Tag] = field(default_factory=list, repr=False)

    # ---- address helpers -------------------------------------------------
    def in_image(self, vaddr: int) -> bool:
        off = vaddr - self.vbase + self.tag_data_offset
        return 0 <= off < len(self.image)

    def off(self, vaddr: int) -> int:
        """virtual address -> offset into the reconstructed image."""
        return vaddr - self.vbase + self.tag_data_offset

    def u32(self, off: int) -> int:
        return struct.unpack_from("<I", self.image, off)[0]

    def u16(self, off: int) -> int:
        return struct.unpack_from("<H", self.image, off)[0]

    def read(self, off: int, n: int) -> bytes:
        return self.image[off:off + n]

    def cstr(self, vaddr: int, limit: int = 256) -> str:
        if not self.in_image(vaddr):
            return ""
        o = self.off(vaddr)
        end = self.image.find(b"\x00", o, o + limit)
        if end < 0:
            end = o + limit
        return self.image[o:end].decode("latin-1", "replace")

    @property
    def map_type_name(self) -> str:
        return MAP_TYPE_NAMES.get(self.map_type, f"type{self.map_type}")

    def tags_by_group(self, group: str) -> list[Tag]:
        return [t for t in self.tags if t.group == group]


def _read_header(data: bytes, path: str):
    if len(data) < HEADER_SIZE:
        raise MapParseError(f"{path}: too small to be a map ({len(data)} bytes)")
    head = struct.unpack_from("<I", data, 0)[0]
    if head != HEAD_LITERAL:
        raise MapParseError(f"{path}: bad head literal {head:#x}")
    version = struct.unpack_from("<I", data, 0x04)[0]
    dec_size = struct.unpack_from("<I", data, 0x08)[0]
    tdoff = struct.unpack_from("<I", data, 0x10)[0]
    tdsize = struct.unpack_from("<I", data, 0x14)[0]
    name = data[0x20:0x40].split(b"\x00", 1)[0].decode("latin-1", "replace")
    build = data[0x40:0x60].split(b"\x00", 1)[0].decode("latin-1", "replace")
    map_type = struct.unpack_from("<I", data, 0x60)[0]
    foot = struct.unpack_from("<I", data, 0x7FC)[0]
    if foot != FOOT_LITERAL:
        raise MapParseError(f"{path}: bad foot literal {foot:#x}")
    return version, dec_size, tdoff, tdsize, name, build, map_type


def _reconstruct_image(data: bytes, dec_size: int):
    """Return (image_bytes, compressed_flag). Handles zlib body or raw."""
    body_start = HEADER_SIZE
    is_zlib = len(data) > body_start + 2 and data[body_start] == 0x78
    if is_zlib:
        try:
            body = zlib.decompress(data[body_start:])
            return data[:HEADER_SIZE] + body, True
        except zlib.error:
            pass  # fall through to raw
    # Uncompressed: the file *is* the image.
    return data, False


def read_map_header(data: bytes, label: str = "<bytes>") -> dict:
    """Header-only parse (no decompression) — enough for an inventory row.
    Needs only the first 2 KB, so callers can pass just the first cluster."""
    version, dec_size, tdoff, tdsize, name, build, map_type = _read_header(data, label)
    return {
        "internal_name": name, "build_stamp": build, "version": version,
        "type": MAP_TYPE_NAMES.get(map_type, f"type{map_type}"),
        "decompressed_size": dec_size,
    }


def parse_map(path: str, *, with_tags: bool = True) -> HaloMap:
    with open(path, "rb") as fh:
        data = fh.read()
    return parse_map_bytes(data, path, with_tags=with_tags)


def parse_map_bytes(data: bytes, path: str = "<bytes>", *, with_tags: bool = True) -> HaloMap:
    version, dec_size, tdoff, tdsize, name, build, map_type = _read_header(data, path)
    image, compressed = _reconstruct_image(data, dec_size)

    m = HaloMap(
        path=path, name=name, build=build, version=version, map_type=map_type,
        decompressed_size=dec_size, tag_data_offset=tdoff, tag_data_size=tdsize,
        compressed=compressed, image=image,
    )

    if tdoff + TAG_INDEX_HEADER_SIZE > len(image):
        raise MapParseError(
            f"{path}: tag_data_offset {tdoff:#x} outside image ({len(image):#x})")

    tih = struct.unpack_from("<9I", image, tdoff)
    tag_array_vaddr, scenario_id, map_id, tag_count = tih[0], tih[1], tih[2], tih[3]
    tags_literal = tih[8]
    if tags_literal != TAGS_LITERAL:
        raise MapParseError(
            f"{path}: 'tags' literal not found at index header "
            f"(got {tags_literal:#x}/{fourcc(tags_literal)!r})")

    # Derive virtual base from the data: the tag array sits 0x24 after the
    # index header, whose file offset is tag_data_offset.
    vbase = tag_array_vaddr - TAG_INDEX_HEADER_SIZE
    m.vbase = vbase
    m.scenario_tag_id = scenario_id
    m.map_id = map_id

    if not with_tags:
        return m

    if tag_count <= 0 or tag_count > 1_000_000:
        raise MapParseError(f"{path}: implausible tag_count {tag_count}")

    arr_off = tdoff + TAG_INDEX_HEADER_SIZE
    need = arr_off + tag_count * TAG_ENTRY_SIZE
    if need > len(image):
        raise MapParseError(
            f"{path}: tag array overruns image (need {need:#x}, have {len(image):#x})")

    tags: list[Tag] = []
    for i in range(tag_count):
        base = arr_off + i * TAG_ENTRY_SIZE
        g0, g1, g2, tid, p_va, meta_va, indexed, _z = struct.unpack_from(
            "<8I", image, base)
        path_str = m.cstr(p_va) if p_va else ""
        tags.append(Tag(
            index=i, group=fourcc(g0), group_raw=g0, tag_id=tid,
            path=path_str, path_vaddr=p_va, meta_vaddr=meta_va, indexed=indexed,
        ))
    m.tags = tags
    return m


if __name__ == "__main__":
    import sys
    import collections
    for p in sys.argv[1:]:
        m = parse_map(p)
        print(f"\n=== {p}")
        print(f"  name={m.name!r} build={m.build!r} type={m.map_type_name} "
              f"version={m.version} compressed={m.compressed}")
        print(f"  vbase={m.vbase:#x} tag_data_offset={m.tag_data_offset:#x} "
              f"tags={len(m.tags)} scenario_id={m.scenario_tag_id:#x}")
        hist = collections.Counter(t.group for t in m.tags)
        top = ", ".join(f"{g}:{c}" for g, c in hist.most_common(12))
        print(f"  groups: {top}")
