"""Local shim: load Halo 2 *Xbox* cache maps with Sigmmma's `reclaimer` on
Python 3.14.

Why this exists
---------------
`reclaimer.meta.wrappers.halo_map.HaloMap.setup_defs()` registers tag
definitions with two *separate* `exec()` calls:

    exec("from %s.%s import get" % (module, fcc))
    exec("defs['%s'] = get()" % fcc)

On Python 3.13+ those two `exec()` calls no longer share an implicit
namespace, so `get` is undefined in the second call and **every** tag def
silently fails to load (`defs == {}` -> every `get_meta()` returns None).

We replace `setup_defs` with an importlib-based loader that keeps a single
namespace. Pure shim: no reclaimer source is edited.
"""
import importlib
from traceback import format_exc

from reclaimer.meta.wrappers import halo_map as _hm
from reclaimer.meta.wrappers.halo_map import HaloMap
from reclaimer.meta.wrappers.halo2_map import Halo2Map

_VALID = _hm.VALID_MODULE_NAME_CHARS


def _module_name(fcc: str) -> str:
    fcc2 = "".join(c if c in _VALID else "_" for c in fcc)
    fcc2 += "_" * ((4 - (len(fcc2) % 4)) % 4)
    return fcc2


def _load_defs(module_path: str, fccs) -> dict:
    defs = {}
    for fcc in fccs:
        try:
            sub = importlib.import_module(f"{module_path}.{_module_name(fcc)}")
        except ImportError:
            continue
        try:
            defs[fcc] = sub.get()
        except Exception:
            print(f"[h2lib] def load failed for {fcc!r}:\n{format_exc()}")
    return defs


def _patched_setup_defs(self):
    cls = type(self)
    assert cls is not HaloMap
    if cls.defs:
        return
    cls.defs = _load_defs(self.tag_defs_module, cls.tag_classes_to_load)


# install the fix on the base class (all wrappers inherit it)
HaloMap.setup_defs = _patched_setup_defs


def open_map(path, extra_classes=()):
    """Open an H2 Xbox cache map. `extra_classes` adds tag fourccs to load
    beyond reclaimer's default set (e.g. 'mode','hlmt','bipd' for models)."""
    if extra_classes:
        base = tuple(Halo2Map.tag_classes_to_load)
        Halo2Map.tag_classes_to_load = base + tuple(
            c for c in extra_classes if c not in base
        )
        Halo2Map.defs = {}  # force a reload with the wider set
    m = Halo2Map()
    m.load_map(str(path))
    return m
