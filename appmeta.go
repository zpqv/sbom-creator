// SPDX-License-Identifier: Apache-2.0

package main

import "strings"

// Curated one-line descriptions for GUI apps. macOS .app bundles carry no
// description field in Info.plist, so we supply these for the well-known
// applications (Apple built-ins plus common third-party). Anything not listed
// simply renders without a description rather than a fabricated one.
var appDesc = map[string]string{
	// Apple built-ins
	"app store": "Apple's marketplace for macOS apps.", "apps": "Apple's app discovery hub.",
	"automator": "Build no-code automation workflows.", "books": "Read and manage ebooks and PDFs.",
	"calculator": "Basic, scientific and programmer calculator.", "calendar": "Manage events and schedules.",
	"chess": "Apple's chess game.", "clock": "World clock, alarms, timers and stopwatch.",
	"contacts": "Address book.", "dictionary": "Dictionary and thesaurus.", "facetime": "Audio and video calling.",
	"findmy": "Locate devices and people.", "font book": "Install and manage fonts.",
	"freeform": "Collaborative whiteboard canvas.", "games": "Apple's games hub.",
	"home": "Control HomeKit smart-home accessories.", "image capture": "Import images from cameras and scanners.",
	"image playground": "AI image generation (Apple Intelligence).", "iphone mirroring": "View and control your iPhone from the Mac.",
	"journal": "Personal journaling app.", "mail": "Email client.", "maps": "Maps and navigation.",
	"messages": "iMessage and SMS.", "mission control": "Window and desktop spaces overview.",
	"music": "Apple Music player.", "news": "News aggregator.", "notes": "Note-taking.",
	"passwords": "Password manager.", "phone": "Make calls via a connected iPhone.",
	"photo booth": "Take photos and video with the webcam.", "photos": "Photo library and editing.",
	"podcasts": "Podcast player.", "preview": "View and annotate PDFs and images.",
	"quicktime player": "Media playback and screen recording.", "reminders": "To-do lists and reminders.",
	"safari": "Apple's web browser.", "shortcuts": "Build and run automation shortcuts.",
	"siri": "Voice assistant.", "stickies": "Desktop sticky notes.", "stocks": "Stock quotes and news.",
	"system settings": "macOS configuration.", "textedit": "Plain and rich text editor.",
	"time machine": "Backup utility.", "tips": "macOS tips.", "tv": "Apple TV video player.",
	"voicememos": "Audio recordings.", "weather": "Weather forecasts.", "keynote": "Apple presentation software.",
	"pages": "Apple word processor.", "numbers": "Apple spreadsheet application.",
	// Common third-party
	"chatgpt": "OpenAI's ChatGPT desktop application.", "claude": "Anthropic's Claude desktop application.",
	"visual studio code": "Microsoft's extensible source-code editor.", "xcode": "Apple's IDE for macOS, iOS, iPadOS, watchOS and tvOS.",
	"google chrome": "Google's web browser.", "microsoft teams": "Team chat, meetings and collaboration.",
	"microsoft word": "Word processor.", "microsoft excel": "Spreadsheet application.",
	"microsoft powerpoint": "Presentation application.", "slack": "Team messaging and collaboration.",
	"zoom": "Video conferencing.", "discord": "Voice, video and text chat.", "spotify": "Music streaming.",
	"docker desktop": "Run and manage Docker containers locally.", "tailscale": "Zero-config mesh VPN built on WireGuard.",
	"ghostty": "Fast, native, GPU-accelerated terminal emulator.", "1password": "Password manager.",
	"notion": "Notes, docs and project workspace.", "obsidian": "Markdown knowledge base.",
	"claude code url handler": "Helper app that handles claude:// deep links for Claude Code.",
}

func descForApp(name string) string {
	return appDesc[strings.ToLower(name)]
}
