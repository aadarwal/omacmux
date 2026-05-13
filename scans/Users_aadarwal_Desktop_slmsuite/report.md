# Scan Report
Generated: 2026-03-31T01:00:00Z

---

# slmsuite Codebase Scan — Executive Summary

**Project:** slmsuite v0.4.1 — GPU-accelerated spatial light modulator control & computational holography library
**Maintainer:** Holodyne Labs, Inc. | **License:** MIT | **~31,000 lines of Python**

---

## 1. Architecture Overview

A three-layer scientific Python library:

- **`holography/`** — Phase retrieval algorithms (Gerchberg-Saxton, WGS), image analysis, Zernike/LG phase patterns, CUDA kernels
- **`hardware/`** — Hardware abstraction for 11 SLM drivers + 15 camera drivers, plus a combined `CameraSLM` integration class
- **`misc/`** — HDF5 utilities, fitting functions, math constants

GPU acceleration via CuPy with automatic NumPy fallback. Optional PyTorch for loss functions.

---

## 2. Critical Findings

### 61 bare `except:` clauses — masks real bugs
The most widespread code quality issue. Found across nearly every module:
- `hardware/cameras/camera.py` (7 instances, lines 244, 482, 511, etc.)
- `hardware/cameras/alliedvision.py` (8 instances)
- `holography/algorithms/_hologram.py` (5 instances)
- `holography/algorithms/_stats.py` (5 instances)

These silently swallow errors during hardware cleanup, image processing, and algorithm execution.

### Mega-files need decomposition
| File | Lines | Methods |
|------|-------|---------|
| `hardware/cameraslms.py` | 4,097 | 100+ |
| `holography/analysis/__init__.py` | 2,405 | — |
| `holography/toolbox/phase.py` | 2,030 | — |
| `holography/algorithms/_hologram.py` | 2,011 | 45 |
| `hardware/cameras/camera.py` | 1,790 | 42 |

### Incomplete feature blocking execution
- `hardware/cameraslms.py:708` — `raise NotImplementedError("TODO")` in a reachable code path

### Hardcoded Windows paths
- `hardware/cameras/thorlabs.py:36-40` — `C:\Program Files\Thorlabs\...`
- `hardware/slms/meadowlark.py:30` — `C:\Program Files\Meadowlark Optics\...`

---

## 3. Security

- No `eval()`/`exec()`/`pickle` abuse found — good
- `hardware/remote.py` has an **unencrypted, unauthenticated** network interface (documented, but still a risk on shared networks)
- 12 wildcard imports (`from X import *`) including `from ctypes import *` in `hamamatsu.py`, `xenics.py`, `_slm_win.py`

---

## 4. Technical Debt

- **31 TODO comments** — 13 in template files (expected), remainder in production code
- **3 deprecated parameter shims** with `warnings.warn()` still active (`cameraslms.py:1515-1522`, `analysis/__init__.py:1316`)
- **Missing type hints** on most public API methods (especially `cameraslms.py`, `camera.py`, `slm.py`)
- **No code coverage metrics** — pytest-cov marked TODO in test README

---

## 5. Strengths

- Comprehensive test suite (12 test files, pytest fixtures for simulated hardware, benchmarking)
- CI on Python 3.10–3.14 via GitHub Actions
- Excellent docstrings (NumPy style, ~95% coverage on public methods)
- Graceful degradation when optional vendor SDKs missing
- Modern tooling: `pyproject.toml`, `uv` package manager
- Template pattern for community hardware contributions

---

## 6. Recommended Actions

| Priority | Action | Impact |
|----------|--------|--------|
| High | Replace bare `except:` with specific exceptions | Prevents masked bugs in hardware and algorithms |
| High | Resolve `NotImplementedError("TODO")` at `cameraslms.py:708` | Unblocks feature |
| Medium | Refactor `cameraslms.py` (4K lines) into submodules | Maintainability |
| Medium | Add return type hints to public APIs | Developer experience, tooling support |
| Medium | Remove wildcard imports (especially `ctypes`) | Namespace clarity, static analysis |
| Low | Make vendor paths configurable (env vars or config) | Cross-platform robustness |
| Low | Enable pytest-cov and set coverage targets | Quality assurance |
