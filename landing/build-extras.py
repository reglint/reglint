#!/usr/bin/env python3
"""Post-build extras: landing copy, sitemap root URL, llms-full.txt."""
import pathlib
import shutil

site = pathlib.Path("site")
docs = pathlib.Path("docs")
landing = pathlib.Path("landing")

# Landing page + OG image (invisible to MkDocs, copied over the build output).
shutil.copy(landing / "index.html", site / "index.html")
shutil.copy(landing / "og.png", site / "og.png")

# The landing is invisible to MkDocs' sitemap; add it.
sm = site / "sitemap.xml"
s = sm.read_text()
sm.write_text(s.replace("</urlset>", "<url><loc>https://reglint.iyaki.ar/</loc></url></urlset>"))

# llms-full.txt: every docs page in one machine-readable fetch.
parts = [
    "# RegLint — full documentation",
    "",
    "> RegLint: regex-based static analysis with rules in plain YAML. Single static Go binary, CI-native output (JSON, SARIF, GitHub PR annotations), git-scoped scans, baselines.",
    "",
    "Landing: https://reglint.iyaki.ar/",
    "",
]
for f in sorted(docs.glob("*.md")):
    url = "https://reglint.iyaki.ar/" + ("" if f.stem == "index" else f.stem + "/")
    parts += ["---", "", f"<!-- source: {url} -->", "", f.read_text(), ""]
(site / "llms-full.txt").write_text("\n".join(parts))
