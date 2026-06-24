#!/usr/bin/env python3
"""Read-only FATX inventory/extract from a raw Xbox HDD image using pyfatx.
Usage:
  fatx_extract.py inventory <raw> [partition=e] [subpath=/UDATA]
  fatx_extract.py pull <raw> <fatx_src_path> <dest_file>
  fatx_extract.py pulltree <raw> <fatx_dir> <dest_dir>
Never writes to <raw> (pyfatx opens read-only here; we only call read paths).
"""
import sys, os
sys.path.insert(0, os.path.expanduser('~/xemu-hdd-extract/venv/lib/python3.14/site-packages'))
from pyfatx import Fatx

def open_e(raw, drive='e'):
    return Fatx(raw, drive=drive)

def inventory(raw, drive='e', sub='/UDATA'):
    fs = open_e(raw, drive)
    n=0
    for dirpath, dirs, files in fs.walk(sub):
        for f in files:
            try:
                a = fs.get_attr(os.path.join(dirpath, f))
                sz = a.file_size
            except Exception as e:
                sz = -1
            print(f"{sz:>10}  {dirpath}/{f}")
            n+=1
        if not files and not dirs:
            print(f"{'<dir>':>10}  {dirpath}/")
    print(f"# total files: {n}", file=sys.stderr)

def pull(raw, src, dest, drive='e'):
    fs = open_e(raw, drive)
    data = bytes(fs.read(src))
    os.makedirs(os.path.dirname(dest), exist_ok=True)
    open(dest,'wb').write(data)
    print(f"wrote {len(data)} bytes -> {dest}")

def pulltree(raw, root, destdir, drive='e'):
    fs = open_e(raw, drive)
    count=0
    for dirpath, dirs, files in fs.walk(root):
        rel = os.path.relpath(dirpath, root)
        outd = os.path.join(destdir, rel) if rel!='.' else destdir
        os.makedirs(outd, exist_ok=True)
        for f in files:
            try:
                data = bytes(fs.read(os.path.join(dirpath, f)))
                open(os.path.join(outd, f),'wb').write(data)
                count+=1
            except Exception as e:
                print(f"  ! fail {dirpath}/{f}: {e}", file=sys.stderr)
    print(f"# extracted {count} files -> {destdir}", file=sys.stderr)

if __name__=='__main__':
    cmd=sys.argv[1]
    if cmd=='inventory':
        inventory(sys.argv[2], *(sys.argv[3:5] and [sys.argv[3]] or []), **({'sub':sys.argv[4]} if len(sys.argv)>4 else {}))
    elif cmd=='pull':
        pull(sys.argv[2], sys.argv[3], sys.argv[4], *(sys.argv[5:] and [sys.argv[5]] or []))
    elif cmd=='pulltree':
        pulltree(sys.argv[2], sys.argv[3], sys.argv[4], *(sys.argv[5:] and [sys.argv[5]] or []))
