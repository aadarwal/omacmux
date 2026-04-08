# Codebase Profile: slmsuite

- **Remote:** https://github.com/aadarwal/slmsuite.git
- **Total files:** 106

## Languages

- Python (.py): 65 files
- png (.png): 11 files
- rst (.rst): 9 files
- Markdown (.md): 5 files
- svg (.svg): 3 files
- YAML (.yaml): 2 files
- YAML (.yml): 1 files
- TOML (.toml): 1 files
- ini (.ini): 1 files
- ico (.ico): 1 files
- gitignore (.gitignore): 1 files
- cu (.cu): 1 files
- csv (.csv): 1 files
- CSS (.css): 1 files
- bat (.bat): 1 files

## Lines of Code by Directory

- .github/: 29 lines
- docs/: 5876 lines
- slmsuite/: 31577 lines
- tests/: 6438 lines

**Total LOC:** 43920

## Directory Structure
```
.
./.git 2
./.github
./.github/workflows
./docs
./docs/source
./slmsuite
./slmsuite 2
./slmsuite/hardware
./slmsuite/holography
./slmsuite/misc
./tests
./tests 2
./tests/hardware
./tests/holography
./tests/misc
```

## Key Config Files

- pyproject.toml
- .github/workflows/

## README (excerpt)

<p align="center">
<picture>
<source media="(prefers-color-scheme: dark)" srcset="https://raw.githubusercontent.com/holodyne/slmsuite/main/docs/source/static/slmsuite-dark.svg">
<img alt="slmsuite" src="https://raw.githubusercontent.com/holodyne/slmsuite/main/docs/source/static/slmsuite.svg" width="256">
</picture>
</p>

<h2 align="center">High-Performance Spatial Light Modulator Control and Holography</h2>

<p align="center">
<a href="https://slmsuite.readthedocs.io/en/latest"><img alt="Documentation Status" src="https://readthedocs.org/projects/slmsuite/badge/?version=latest"></a>
<a href="https://pypi.org/project/slmsuite/"><img alt="PyPi Package" src="https://img.shields.io/pypi/v/slmsuite"></a>
<a href="https://github.com/holodyne/slmsuite/blob/main/LICENSE"><img alt="License: MIT" src="https://img.shields.io/github/license/holodyne/slmsuite?color=purple"></a>
<a href="https://github.com/psf/black"><img alt="Code style: black" src="https://img.shields.io/badge/code%20style-black-000000.svg"></a>
</p>

`slmsuite` combines GPU-accelerated beamforming algorithms with optimized hardware control, automated calibration, and user-friendly scripting to enable high-performance programmable optics with modern spatial light modulators.

## Key Features
- [GPU-accelerated iterative phase retrieval  algorithms](https://slmsuite.readthedocs.io/en/latest/_examples/computational_holography.html#Computational-Holography) (e.g. Gerchberg-Saxton, weighted GS, or phase-stationary WGS)
- [A simple hardware-control interface](https://slmsuite.readthedocs.io/en/latest/_examples/experimental_holography.html#Loading-Hardware) for working with various SLMs and cameras
- [Automated Fourier- to image-space coordinate transformations](https://slmsuite.readthedocs.io/en/latest/_examples/experimental_holography.html#Fourier-Calibration): choose how much light goes to which camera pixels; `slmsuite` takes care of the rest!
- [Automated wavefront calibration](https://slmsuite.readthedocs.io/en/latest/_examples/wavefront_calibration.html) to improve manufacturer-supplied flatness maps or compensate for additional aberrations along the SLM imaging train
- Optimized [optical focus/spot arrays](https://slmsuite.readthedocs.io/en/latest/_examples/computational_holography.html#Spot-Arrays) using [camera feedback](https://slmsuite.readthedocs.io/en/latest/_examples/experimental_holography.html#A-Uniform-Square-Array), automated statistics, and numerous analysis routines
- [Mixed region amplitude freedom](https://slmsuite.readthedocs.io/en/latest/_autosummary/slmsuite.holography.algorithms.Hologram.html#slmsuite.holography.algorithms.Hologram.optimize), which ignores unused far-field regions in favor of optimized hologram performance in high-interest areas.
- [Toolboxes for structured light](https://slmsuite.readthedocs.io/en/latest/_examples/structured_light.html#), imprinting sectioned phase masks, SLM unit conversion, padding and unpadding data, and more
- A fully-featured [example library](https://slmsuite.readthedocs.io/en/latest/examples.html) that demonstrates these and other features

## Installation

Install the stable version of `slmsuite` from [PyPI](https://pypi.org/project/slmsuite/) using:

```console
pip install slmsuite
```

Install the latest version of `slmsuite` from GitHub using:

```console
pip install git+https://github.com/holodyne/slmsuite
```

## Documentation and Examples

Extensive
[documentation](https://slmsuite.readthedocs.io/en/latest/)
and
[API reference](https://slmsuite.readthedocs.io/en/latest/api.html)
are available through readthedocs.


## Recent Commits
```
2bb94f5 Merge pull request #177 from sumiya-kuroda/fix-zernike-variable
cdb6400 Increment to 0.4.1
70cc2a2 Assorted bugfixes, documentation, `zernike_order_number`.
c3fe41a Remove undefined local variable (#176)
7a363db Merge pull request #175 from holodyne/holodyne-xfer
a76ed99 Readme update.
9ce9645 Update README_PYPI.md
29dc334 Merge branch 'holodyne-xfer' of https://github.com/slmsuite/slmsuite into holodyne-xfer
6b882c6 Transfer cleanup and minor bugfixes.
f277d03 Merge branch 'holodyne-xfer' of https://github.com/holodyne/slmsuite into holodyne-xfer
ae72106 Revert phase storage.
8b01015 Update readme
4bd3e73 Remove citation; update install instructions.
3d6bbd2 Fixed issue with separate files for notebook download.
8af4fff Small cleanup and polish to zernike calibration
9b1fcf8 Minor plotting/gif updates, flir adc update
a84765f Merge branch 'holodyne-xfer' of https://github.com/holodyne/slmsuite into holodyne-xfer
c2c1268 Bugfixes to array detection, wavefront calibration.
b886d21 Fix #169 Blink 1.1.4.124
e1797e7 Reverted name changes.
```
