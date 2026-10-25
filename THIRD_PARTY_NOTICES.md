# Third-party notices

OhMyMC links the following pinned libraries. The original license texts are copied without modification into `licenses/` and must accompany redistributed binaries. Dependency versions are authoritative in `go.mod` and `go.sum`.

| Component | Pinned version | License / included notice |
| --- | --- | --- |
| [moejs](https://github.com/Calcium-Ion/moejs) | v0.1.0-alpha.10 | Apache-2.0; `moejs-LICENSE.txt` and **`moejs-THIRD_PARTY_NOTICES.txt`** |
| [MyGO](https://github.com/egoist/mygo) | v0.6.4-0.20261025001514-9c69cf829ece | MIT; `mygo-LICENSE.txt`; shader-derived code notice in `mygo-shader-LICENSE.txt` |
| [purego](https://github.com/ebitengine/purego) | v0.11.1 | Apache-2.0; `purego-LICENSE.txt` |
| [go-text/typesetting](https://github.com/go-text/typesetting) | v0.3.5 | Unlicense OR BSD-3-Clause; `typesetting-LICENSE.txt`; HarfBuzz notices in `typesetting-harfbuzz-LICENSE.txt` |
| golang.org/x/image | v0.46.0 | BSD-3-Clause; `golang-x-image-LICENSE.txt` |
| golang.org/x/net | v0.59.0 | BSD-3-Clause; `golang-x-net-LICENSE.txt` |
| golang.org/x/text | v0.43.0 | BSD-3-Clause; `golang-x-text-LICENSE.txt` |
| Go runtime / standard library | built with Go 1.27.3 | BSD-3-Clause; `go-LICENSE.txt` |

moejs's complete upstream third-party notice is retained, including the Unicode data, ICU, and V8 notices required for compiled distributions. We do not bundle moejs's external AGPL benchmark plugins. Test-only Go dependencies are resolved by Go modules; no test corpus or downloaded Minecraft client is distributed with the launcher.

WebView2 is a separately installed Microsoft runtime, governed by its own license; it is not bundled here. Java and Minecraft are also obtained separately. Minecraft is a trademark of Mojang/Microsoft; OhMyMC is not affiliated with or endorsed by them.

The project-local SVG artwork and static UI were created for this implementation and do not fetch third-party fonts, illustrations, icons or scripts from CDNs. Original OhMyMC code and assets are licensed under the MIT License in the root `LICENSE` file. This does not replace or relicense any third-party component; the original notices above must be retained.
