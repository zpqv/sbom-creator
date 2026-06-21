// SPDX-License-Identifier: Apache-2.0

package main

import (
	"net/url"
	"regexp"
	"strings"
)

// ---------------------------------------------------------------------------
// Category classification
//
// Strategy, in priority order:
//  1. exact name override (well-known tools, stable across machines)
//  2. Homebrew formula map (precise, since brew names are canonical)
//  3. keyword heuristics over name+description
//  4. source-based default (pip -> Python library, etc.)
// ---------------------------------------------------------------------------

var nameCat = map[string]string{
	// AI / LLM tooling
	"claude": "AI / LLM Tools", "claude-code": "AI / LLM Tools", "chatgpt": "AI / LLM Tools",
	"codex": "AI / LLM Tools", "cline": "AI / LLM Tools", "ollama": "AI / LLM Tools",
	"gemini-cli": "AI / LLM Tools", "copilot": "AI / LLM Tools", "copilot-cli": "AI / LLM Tools",
	"qwen-code": "AI / LLM Tools", "cursor": "AI / LLM Tools", "perplexity": "AI / LLM Tools",
	// Browsers
	"google chrome": "Browsers", "chromium": "Browsers", "firefox": "Browsers", "safari": "Browsers",
	"brave browser": "Browsers", "microsoft edge": "Browsers", "arc": "Browsers", "opera": "Browsers",
	// Editors / IDEs / dev tools
	"visual studio code": "Developer Tools", "code": "Developer Tools", "xcode": "Developer Tools",
	"sublime text": "Developer Tools", "jetbrains toolbox": "Developer Tools", "android studio": "Developer Tools",
	"git": "Version Control", "gh": "Version Control", "git-lfs": "Version Control", "git lfs": "Version Control",
	"cmake": "Developer Tools", "llvm": "Developer Tools", "jq": "Developer Tools", "ripgrep": "Developer Tools",
	"playwright": "Developer Tools", "pandoc": "Developer Tools", "graphviz": "Developer Tools",
	// Terminals
	"warp": "Terminals", "ghostty": "Terminals", "iterm": "Terminals", "iterm2": "Terminals",
	"alacritty": "Terminals", "kitty": "Terminals", "windows terminal": "Terminals",
	// Languages & runtimes
	"go": "Languages & Runtimes", "rust": "Languages & Runtimes", "node": "Languages & Runtimes",
	"ruby": "Languages & Runtimes", "flutter": "Languages & Runtimes", "openjdk": "Languages & Runtimes",
	// Package managers
	"npm": "Package Managers", "pip": "Package Managers", "uv": "Package Managers",
	"cocoapods": "Package Managers", "yarn": "Package Managers", "pnpm": "Package Managers",
	// Cloud / DevOps
	"firebase-tools": "Cloud & DevOps CLI", "gcloud-cli": "Cloud & DevOps CLI", "azure-cli": "Cloud & DevOps CLI",
	"awscli": "Cloud & DevOps CLI", "aws-cli": "Cloud & DevOps CLI", "kubectl": "Cloud & DevOps CLI",
	"terraform": "Cloud & DevOps CLI", "stripe": "Cloud & DevOps CLI",
	// Containers & virtualization
	"orbstack": "Containers & Virtualization", "docker": "Containers & Virtualization",
	"docker desktop": "Containers & Virtualization", "podman": "Containers & Virtualization",
	"virtualbox": "Containers & Virtualization", "utm": "Containers & Virtualization",
	// Productivity / office / communication
	"libreoffice": "Productivity & Office", "microsoft word": "Productivity & Office",
	"microsoft excel": "Productivity & Office", "microsoft powerpoint": "Productivity & Office",
	"keynote": "Productivity & Office", "pages": "Productivity & Office", "numbers": "Productivity & Office",
	"notion": "Productivity & Office", "obsidian": "Productivity & Office",
	"microsoft teams": "Communication", "slack": "Communication", "zoom": "Communication",
	"discord": "Communication", "telegram": "Communication", "whatsapp": "Communication",
	// Security / networking
	"tailscale": "Networking & Security", "wireguard": "Networking & Security",
	"1password": "Networking & Security", "openssl@3": "Cryptography & Security", "gnupg": "Cryptography & Security",
}

// Canonical Homebrew formula categories (names are stable upstream).
var formulaCat = map[string]string{
	"ada-url": "Networking & Protocols", "aom": "Media & Codecs", "azure-cli": "Cloud & DevOps CLI",
	"brotli": "Compression", "c-ares": "Networking & Protocols", "ca-certificates": "Cryptography & Security",
	"cairo": "Image & Graphics", "cmake": "Developer Tools", "cocoapods": "Package Managers", "dav1d": "Media & Codecs",
	"dotnet@8": "Languages & Runtimes", "ffmpeg": "Media & Codecs", "fmt": "Developer Libraries", "fontconfig": "Fonts & Text",
	"freetype": "Fonts & Text", "fribidi": "Fonts & Text", "gd": "Image & Graphics", "gdbm": "Databases",
	"gdk-pixbuf": "Image & Graphics", "gemini-cli": "AI / LLM Tools", "gettext": "Internationalization", "gh": "Version Control",
	"giflib": "Image & Graphics", "git": "Version Control", "git-lfs": "Version Control", "glib": "Developer Libraries",
	"gmp": "Developer Libraries", "gnupg": "Cryptography & Security", "gnutls": "Cryptography & Security", "go": "Languages & Runtimes",
	"gpgme": "Cryptography & Security", "gpgmepp": "Cryptography & Security", "graphite2": "Fonts & Text", "graphviz": "Developer Tools",
	"gts": "Developer Libraries", "harfbuzz": "Fonts & Text", "hdrhistogram_c": "Developer Libraries", "icu4c@78": "Internationalization",
	"jasper": "Image & Graphics", "jpeg-turbo": "Image & Graphics", "jq": "Developer Tools", "lame": "Media & Codecs",
	"libassuan": "Cryptography & Security", "libavif": "Image & Graphics", "libdatrie": "Fonts & Text", "libevent": "Networking & Protocols",
	"libffi": "Developer Libraries", "libgcrypt": "Cryptography & Security", "libgit2": "Version Control", "libgpg-error": "Cryptography & Security",
	"libidn2": "Internationalization", "libksba": "Cryptography & Security", "libnghttp2": "Networking & Protocols", "libnghttp3": "Networking & Protocols",
	"libngtcp2": "Networking & Protocols", "libpng": "Image & Graphics", "librsvg": "Image & Graphics", "libsodium": "Cryptography & Security",
	"libssh2": "Networking & Protocols", "libtasn1": "Cryptography & Security", "libthai": "Fonts & Text", "libtiff": "Image & Graphics",
	"libtool": "Developer Tools", "libunistring": "Internationalization", "libusb": "Developer Libraries", "libuv": "Networking & Protocols",
	"libvmaf": "Media & Codecs", "libvpx": "Media & Codecs", "libx11": "X11 / Windowing", "libxau": "X11 / Windowing",
	"libxcb": "X11 / Windowing", "libxdmcp": "X11 / Windowing", "libxext": "X11 / Windowing", "libxrender": "X11 / Windowing",
	"libyaml": "Developer Libraries", "little-cms2": "Image & Graphics", "llhttp": "Networking & Protocols", "llvm": "Developer Tools",
	"lz4": "Compression", "lzo": "Compression", "m4": "Developer Tools", "mas": "Developer Tools", "merve": "Developer Libraries",
	"mpdecimal": "Developer Libraries", "nbytes": "Developer Libraries", "netpbm": "Image & Graphics", "nettle": "Cryptography & Security",
	"node": "Languages & Runtimes", "node@20": "Languages & Runtimes", "npth": "Developer Libraries", "nspr": "Developer Libraries",
	"nss": "Cryptography & Security", "oniguruma": "Developer Libraries", "openjdk@21": "Languages & Runtimes", "openjpeg": "Image & Graphics",
	"openssl@3": "Cryptography & Security", "opus": "Media & Codecs", "p11-kit": "Cryptography & Security", "pandoc": "Developer Tools",
	"pango": "Fonts & Text", "pcre2": "Developer Libraries", "pinentry": "Cryptography & Security", "pixman": "Image & Graphics",
	"pkgconf": "Developer Tools", "poppler": "Image & Graphics", "python@3.10": "Languages & Runtimes", "python@3.13": "Languages & Runtimes",
	"python@3.14": "Languages & Runtimes", "readline": "Developer Libraries", "ripgrep": "Developer Tools", "ruby": "Languages & Runtimes",
	"rust": "Languages & Runtimes", "sdl2-compat": "Media & Codecs", "sdl3": "Media & Codecs", "simdjson": "Developer Libraries",
	"simdutf": "Developer Libraries", "sqlite": "Databases", "stripe": "Cloud & DevOps CLI", "svt-av1": "Media & Codecs",
	"unbound": "Networking & Protocols", "uv": "Package Managers", "uvwasi": "Developer Libraries", "webp": "Image & Graphics",
	"wget": "Networking & Protocols", "x264": "Media & Codecs", "x265": "Media & Codecs", "xorgproto": "X11 / Windowing",
	"xz": "Compression", "z3": "Developer Tools", "zstd": "Compression",
}

type kwRule struct {
	cat string
	kw  []string
}

// Ordered so more specific buckets win before generic ones. Keywords are
// matched on word/phrase boundaries (see kwMatchers), not raw substrings, so
// e.g. "ssl" does not match inside "lossless".
var kwRules = []kwRule{
	{"Cryptography & Security", []string{"crypto", "cryptographic", "cryptography", "openssl", "ssl", "tls", "cipher", "encryption", "gpg", "pgp", "x.509", "x509", "certificate", "keychain", "password", "vault", "asn.1", "pkcs"}},
	{"Compression", []string{"compress", "compression", "decompress", "decompression", "zip", "gzip", "lossless compression"}},
	{"Media & Codecs", []string{"codec", "video", "audio", "h.264", "h.265", "av1", "mp3", "encoder", "decoder", "multimedia", "ffmpeg"}},
	{"Fonts & Text", []string{"font", "fonts", "glyph", "text shaping", "typeface", "bidi", "opentype"}},
	{"Image & Graphics", []string{"image", "imaging", "png", "jpeg", "tiff", "svg", "graphics", "pixel", "raster", "pdf render", "color management"}},
	{"Internationalization", []string{"unicode", "i18n", "l10n", "localization", "internationalization", "locale"}},
	{"Networking & Protocols", []string{"http", "dns", "tcp", "quic", "network", "networking", "socket", "url parser", "rpc", "grpc", "download", "ssh"}},
	{"Databases", []string{"database", "sql", "sqlite", "key-value store", "datastore"}},
	{"Developer Tools", []string{"compiler", "build", "linker", "debugger", "lint", "formatter", "ide", "editor", "language server"}},
	{"Cloud & DevOps CLI", []string{"cloud", "kubernetes", "aws", "azure", "gcp", "terraform", "deploy", "ci/cd"}},
	{"Languages & Runtimes", []string{"programming language", "runtime", "interpreter", "jdk", "sdk for"}},
	{"AI / LLM Tools", []string{"llm", "ai assistant", "ai-powered", "coding agent", "language model", "gpt", "chatbot"}},
}

// kwMatchers holds one boundary-anchored regex per rule, compiled once. A
// keyword matches only when bounded by a non-alphanumeric character (or string
// edge) on both sides, giving whole-word/phrase semantics.
var kwMatchers = func() []struct {
	cat string
	re  *regexp.Regexp
} {
	out := make([]struct {
		cat string
		re  *regexp.Regexp
	}, len(kwRules))
	for i, r := range kwRules {
		alts := make([]string, len(r.kw))
		for j, k := range r.kw {
			alts[j] = regexp.QuoteMeta(k)
		}
		out[i].cat = r.cat
		out[i].re = regexp.MustCompile(`(?i)(^|[^a-z0-9])(` + strings.Join(alts, "|") + `)([^a-z0-9]|$)`)
	}
	return out
}()

func classifyCategory(c Component) string {
	low := strings.ToLower(c.Name)
	if v, ok := nameCat[low]; ok {
		return v
	}
	if c.Source == "Homebrew (formula)" {
		if v, ok := formulaCat[low]; ok {
			return v
		}
	}
	hay := c.Name + " " + c.Desc
	for _, m := range kwMatchers {
		if m.re.MatchString(hay) {
			return m.cat
		}
	}
	switch c.Source {
	case "pip (Python)":
		return "Python Library"
	case "gem (Ruby)":
		return "Ruby Library"
	case "cargo (Rust)":
		return "Developer Tools"
	case "Homebrew (formula)":
		return "Developer Libraries"
	case "macOS (built-in)":
		return "macOS (built-in)"
	}
	return "Applications"
}

// ---------------------------------------------------------------------------
// Vendor / provider inference from a homepage URL
// ---------------------------------------------------------------------------

var vendorOverride = map[string]string{
	"google": "Google", "googlesource": "Google", "webmproject": "Google", "go.dev": "Google (Go team)",
	"flutter.dev": "Google", "firebase": "Google (Firebase)", "cloud.google.com": "Google Cloud",
	"geminicli.com": "Google", "gnu.org": "GNU Project", "gnupg.org": "GnuPG Project",
	"x.org": "X.Org Foundation", "videolan.org": "VideoLAN", "mozilla": "Mozilla",
	"nodejs.org": "Node.js (OpenJS Foundation)", "gtk.org": "GNOME Project", "gnome.org": "GNOME Project",
	"freedesktop.org": "freedesktop.org", "microsoft.com": "Microsoft", "dotnet.microsoft.com": "Microsoft",
	"apple.com": "Apple", "icu.unicode.org": "Unicode Consortium", "openssl-library.org": "OpenSSL Project",
	"sqlite.org": "SQLite (Hwaci)", "python.org": "Python Software Foundation", "ruby-lang.org": "Ruby core team",
	"rust-lang.org": "Rust Foundation", "astral.sh": "Astral", "facebook.github.io": "Meta",
	"cairographics.org": "Cairo project", "ffmpeg.org": "FFmpeg project", "pcre.org": "PCRE project",
	"libsdl.org": "SDL / libsdl-org", "graphviz.org": "Graphviz", "cli.github.com": "GitHub",
	"docs.github.com": "GitHub", "docs.npmjs.com": "npm, Inc. (GitHub)", "git-scm.com": "Git project",
	"git-lfs.com": "GitHub", "curl.se": "curl / Mozilla CA", "nghttp2.org": "nghttp2 project",
	"libuv.org": "libuv (OpenJS Foundation)", "unbound.net": "NLnet Labs", "tukaani.org": "Tukaani project",
	"openjdk.org": "Oracle / OpenJDK", "chromium.org": "The Chromium Project", "libreoffice.org": "The Document Foundation",
	"ollama.com": "Ollama", "orbstack.dev": "OrbStack", "warp.dev": "Warp", "openai": "OpenAI",
	"anthropics": "Anthropic", "anthropic.com": "Anthropic", "claude.ai": "Anthropic", "cline.bot": "Cline",
	"tailscale.com": "Tailscale", "code.visualstudio.com": "Microsoft", "ghostty.org": "Ghostty project",
	"developer.apple.com": "Apple", "grpc.io": "Google (gRPC)", "stripe.com": "Stripe",
}

var ghOrgRe = regexp.MustCompile(`^(?:github|gitlab|bitbucket)\.com/([^/]+)`)

func vendorFromHomepage(hp string) string {
	if hp == "" {
		return ""
	}
	u, err := url.Parse(hp)
	if err != nil || u.Host == "" {
		return ""
	}
	host := strings.TrimPrefix(strings.ToLower(u.Host), "www.")
	// Override matching is case-insensitive (lowercased path); org extraction
	// preserves the original case so "AOMediaCodec" doesn't become lowercase.
	lowFull := host + strings.ToLower(u.Path)
	for key, val := range vendorOverride {
		if strings.Contains(lowFull, key) {
			return val
		}
	}
	if m := ghOrgRe.FindStringSubmatch(host + u.Path); m != nil {
		return m[1]
	}
	parts := strings.Split(host, ".")
	if len(parts) >= 2 {
		return strings.Join(parts[len(parts)-2:], ".")
	}
	return host
}
