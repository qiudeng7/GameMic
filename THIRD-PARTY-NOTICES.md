# Third-party software

GameMic's own source is MIT licensed. Third-party licenses remain separate.

| Component | License / source |
| --- | --- |
| malgo | Unlicense — https://github.com/gen2brain/malgo |
| miniaudio (bundled by malgo) | Public domain or MIT-0 — https://miniaud.io/ |
| Walk / Win | BSD-3-Clause — https://github.com/lxn/walk and https://github.com/lxn/win |
| golang.org/x/sys | BSD-3-Clause — https://go.googlesource.com/sys |
| govaluate | MIT — https://github.com/Knetic/govaluate |

The release package includes the license files found in these Go modules.

## VB-CABLE

VB-CABLE © VB-Audio Software / Vincent Burel. It is proprietary donationware,
not part of GameMic's MIT-licensed source. The original base driver package is
included unchanged in the Windows installer, with attribution and a way to
donate / purchase a license. It is fetched only while building the installer;
GameMic never downloads or uploads audio during normal use.

- Origin and download: https://vb-audio.com/Cable/
- VB-CABLE is donationware; all participations are welcome.
- License and donation information: https://vb-audio.com/Services/licensing.htm
- Vendor distribution terms permit bundling the base VB-CABLE package when the
  donationware model remains applicable. Professional deployments can require
  separate paid licensing; consult the vendor's terms.

VB-CABLE A+B and C+D are not bundled. The official installer and driver files
are not modified. The installer archive SHA256 and Authenticode signer are
recorded in `licenses/VBCABLE-PROVENANCE.txt` in each built package.
