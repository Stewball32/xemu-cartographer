#!/usr/bin/env python3
"""Read/write a single file on a FATX partition of a raw Xbox HDD image/device.

A general-purpose companion to fatx_console_name.py. Used by the Go podman code
(internal/podman/master_autolaunch.go) against a raw block view of a qcow2
(exposed with qemu-storage-daemon's FUSE export), so writes land in whatever
layer the exposed qcow2 represents. Xbox/format knowledge stays in Go; this
helper is a thin pyfatx shim that moves bytes to/from a FATX path.

Usage:
    fatx_file.py read  <raw-image-or-device> <drive> <fatx-path>
        -> writes the file's raw bytes to stdout (binary). Exit 0 on success.

    fatx_file.py write <raw-image-or-device> <drive> <fatx-path> <base64-bytes>
        -> creates the file if absent (FATX write-CREATE) or overwrites it,
           then truncates to the exact payload length. Exit 0 on success.

<drive> is a FATX drive letter (c, e, f, g, x, y, z). <fatx-path> is the path
within that drive using forward slashes and a leading slash, e.g.
/Dashboard/config.xml.

Requires: pyfatx (`pip install pyfatx`). Exits non-zero with a clear message on
any failure so the caller can log a warning and continue.
"""
import base64
import sys


def _open(raw, drive):
    import pyfatx

    return pyfatx.Fatx(raw, drive=drive)


def do_read(raw, drive, path):
    fs = _open(raw, drive)
    data = fs.read(path)
    sys.stdout.buffer.write(data)
    sys.stdout.buffer.flush()
    return 0


def do_write(raw, drive, path, b64):
    payload = base64.b64decode(b64)
    fs = _open(raw, drive)
    fs.write(path, payload)
    try:
        fs.truncate(path, len(payload))
    except Exception:  # noqa: BLE001
        pass  # truncate is best-effort; payload already fully overwrote
    rb = fs.read(path)
    if rb[: len(payload)] != payload:
        sys.stderr.write("readback mismatch after write\n")
        return 6
    sys.stderr.write(f"wrote {path} ({len(payload)} bytes) OK\n")
    return 0


def main(argv):
    if len(argv) < 2:
        sys.stderr.write(__doc__)
        return 2
    op = argv[1]
    try:
        import pyfatx  # noqa: F401
    except ImportError:
        sys.stderr.write("pyfatx not installed (pip install pyfatx)\n")
        return 3

    try:
        if op == "read":
            if len(argv) != 5:
                sys.stderr.write("usage: fatx_file.py read <raw> <drive> <path>\n")
                return 2
            return do_read(argv[2], argv[3], argv[4])
        if op == "write":
            if len(argv) != 6:
                sys.stderr.write("usage: fatx_file.py write <raw> <drive> <path> <b64>\n")
                return 2
            return do_write(argv[2], argv[3], argv[4], argv[5])
    except Exception as e:  # noqa: BLE001
        sys.stderr.write(f"fatx {op} failed: {e}\n")
        return 5

    sys.stderr.write(f"unknown op {op!r}\n")
    return 2


if __name__ == "__main__":
    sys.exit(main(sys.argv))
