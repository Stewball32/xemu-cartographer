#!/usr/bin/env python3
"""Make the two H2 system-link eeproms with DISTINCT serials (+distinct MAC).

THE H2 SYSTEM-LINK IDENTITY FIX (verified 2026-07-02): two xemu instances that
clone the same base disk + share the eeprom SERIAL are treated by Halo 2 as the
SAME console, so the client filters the host's game out of AVAILABLE GAMES even
though the reflector bridges the discovery frames. Giving each instance a DISTINCT
eeprom serial makes H2 list + join the host's game. (CE did NOT need this.)

Serial lives at eeprom 0x34 (12 ASCII); the factory checksum at 0x30 covers dwords
[0x34:0x5C] and MUST be recomputed (same algo as fleet/make_eeproms.py). MAC @0x40.
"""
import struct, os
D = "/home/stew/.local/share/xemu/xemu"
CK, SER, MAC, LO, HI = 0x30, 0x34, 0x40, 0x34, 0x5C
def cksum(d):
    lo = hi = 0
    for off in range(LO, HI, 4):
        lo += struct.unpack_from("<I", d, off)[0]; hi += lo >> 32; lo &= 0xFFFFFFFF
    return (~(hi + lo)) & 0xFFFFFFFF
def build(src, dst, serial, mac_last):
    d = bytearray(open(os.path.join(D, src), "rb").read())
    d[SER:SER+12] = serial.encode("ascii")[:12].ljust(12, b"0")
    d[MAC:MAC+6] = bytes([0x00,0x50,0xF2,0xC2,0xAF, mac_last])
    struct.pack_into("<I", d, CK, cksum(d))
    open(os.path.join(D, dst), "wb").write(d)
    print(f"  {dst}: serial={d[SER:SER+12].decode()} MAC=..:af:{mac_last:02x} cksum=0x{struct.unpack_from('<I',d,CK)[0]:08x}")
if __name__ == "__main__":
    build("eeprom3.bin", "eeprom-h2sl-1.bin", "757934831097", 0x03)  # inst1: original serial
    build("eeprom4.bin", "eeprom-h2sl-2.bin", "757934830002", 0x04)  # inst2: DISTINCT serial
