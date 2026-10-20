package main

import "github.com/egoist/mygo/ui"

// The icons the interface draws: 24-unit strokes on a 24-unit box, one
// weight, rounded caps. A glyph says what a control does; the label says
// the rest.

func icon(shapes string) *ui.SVG {
	return ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round">` + shapes + `</svg>`))
}

var (
	iconDashboard = icon(`<rect x="3.5" y="3.5" width="7" height="7" rx="1.6"/><rect x="13.5" y="3.5" width="7" height="7" rx="1.6"/><rect x="3.5" y="13.5" width="7" height="7" rx="1.6"/><rect x="13.5" y="13.5" width="7" height="7" rx="1.6"/>`)
	iconFiles     = icon(`<path d="M3.5 7.5A2 2 0 0 1 5.5 5.5h3.2a2 2 0 0 1 1.4.6l1 1a2 2 0 0 0 1.4.6h5.5a2 2 0 0 1 2 2v6.8a2 2 0 0 1-2 2H5.5a2 2 0 0 1-2-2z"/>`)
	iconRecent    = icon(`<circle cx="12" cy="12" r="8.5"/><path d="M12 7.5V12l3 1.8"/>`)
	iconSettings  = icon(`<circle cx="12" cy="12" r="3"/><path d="M19.4 15a1.6 1.6 0 0 0 .3 1.8l.1.1a2 2 0 1 1-2.8 2.8l-.1-.1a1.6 1.6 0 0 0-1.8-.3 1.6 1.6 0 0 0-1 1.5V21a2 2 0 1 1-4 0v-.1a1.6 1.6 0 0 0-1-1.5 1.6 1.6 0 0 0-1.8.3l-.1.1a2 2 0 1 1-2.8-2.8l.1-.1a1.6 1.6 0 0 0 .3-1.8 1.6 1.6 0 0 0-1.5-1H3a2 2 0 1 1 0-4h.1a1.6 1.6 0 0 0 1.5-1 1.6 1.6 0 0 0-.3-1.8l-.1-.1a2 2 0 1 1 2.8-2.8l.1.1a1.6 1.6 0 0 0 1.8.3H9a1.6 1.6 0 0 0 1-1.5V3a2 2 0 1 1 4 0v.1a1.6 1.6 0 0 0 1 1.5 1.6 1.6 0 0 0 1.8-.3l.1-.1a2 2 0 1 1 2.8 2.8l-.1.1a1.6 1.6 0 0 0-.3 1.8V9a1.6 1.6 0 0 0 1.5 1H21a2 2 0 1 1 0 4h-.1a1.6 1.6 0 0 0-1.5 1z"/>`)
	iconSliders   = icon(`<path d="M4 8h10M18 8h2M4 16h4M12 16h8"/><circle cx="16" cy="8" r="2.2"/><circle cx="10" cy="16" r="2.2"/>`)

	iconBucket = icon(`<ellipse cx="12" cy="6" rx="7.5" ry="2.8"/><path d="M4.5 6v6c0 1.5 3.4 2.8 7.5 2.8s7.5-1.3 7.5-2.8V6"/><path d="M4.5 12v6c0 1.5 3.4 2.8 7.5 2.8s7.5-1.3 7.5-2.8v-6"/>`)

	iconUpload   = icon(`<path d="M12 15.5V4M7.5 8.5 12 4l4.5 4.5"/><path d="M4.5 15v3.5a2 2 0 0 0 2 2h11a2 2 0 0 0 2-2V15"/>`)
	iconDownload = icon(`<path d="M12 4v11.5M7.5 11.5 12 16l4.5-4.5"/><path d="M4.5 15v3.5a2 2 0 0 0 2 2h11a2 2 0 0 0 2-2V15"/>`)
	iconPlus     = icon(`<path d="M12 5v14M5 12h14"/>`)
	iconRefresh  = icon(`<path d="M20 12a8 8 0 1 1-2.4-5.7"/><path d="M20 4v4.5h-4.5"/>`)
	iconClose    = icon(`<path d="M6 6l12 12M18 6 6 18"/>`)
	iconCheck    = icon(`<path d="M20 6.5 9.2 17.3 4 12.1"/>`)
	iconSparkle  = icon(`<path d="M12 3.5 13.7 9 19 10.7 13.7 12.4 12 18l-1.7-5.6L5 10.7 10.3 9z"/><path d="M18.5 16.5 19 18l1.5.5-1.5.5-.5 1.5-.5-1.5L16.5 19l1.5-.5z"/>`)

	iconGrid   = icon(`<rect x="3.5" y="3.5" width="7" height="7" rx="1.4"/><rect x="13.5" y="3.5" width="7" height="7" rx="1.4"/><rect x="3.5" y="13.5" width="7" height="7" rx="1.4"/><rect x="13.5" y="13.5" width="7" height="7" rx="1.4"/>`)
	iconList   = icon(`<path d="M8.5 6.5h12M8.5 12h12M8.5 17.5h12"/><path d="M4 6.5h.01M4 12h.01M4 17.5h.01"/>`)
	iconSearch = icon(`<circle cx="10.8" cy="10.8" r="6.8"/><path d="m20 20-4.4-4.4"/>`)
	iconMore   = icon(`<circle cx="5" cy="12" r="1.5" fill="currentColor" stroke="none"/><circle cx="12" cy="12" r="1.5" fill="currentColor" stroke="none"/><circle cx="19" cy="12" r="1.5" fill="currentColor" stroke="none"/>`)

	iconUp          = icon(`<path d="M12 19.5V5M5.5 11.5 12 5l6.5 6.5"/>`)
	iconChevron     = icon(`<path d="m9.5 6 6 6-6 6"/>`)
	iconChevronDown = icon(`<path d="m6 9.5 6 6 6-6"/>`)
	iconChevronUp   = icon(`<path d="m6 14.5 6-6 6 6"/>`)
	iconSort        = icon(`<path d="M8 4v16M8 20l-3.5-3.5M8 20l3.5-3.5"/><path d="M16 20V4M16 4l-3.5 3.5M16 4l3.5 3.5"/>`)
	iconNewFolder   = icon(`<path d="M3.5 7.5A2 2 0 0 1 5.5 5.5h3.2a2 2 0 0 1 1.4.6l1 1a2 2 0 0 0 1.4.6h5.5a2 2 0 0 1 2 2v6.8a2 2 0 0 1-2 2H5.5a2 2 0 0 1-2-2z"/><path d="M12 10.5v5M9.5 13h5"/>`)
	iconShare       = icon(`<path d="M10.5 13.5a4.6 4.6 0 0 0 6.5 0l2.5-2.5a4.6 4.6 0 0 0-6.5-6.5l-1 1"/><path d="M13.5 10.5a4.6 4.6 0 0 0-6.5 0L4.5 13a4.6 4.6 0 0 0 6.5 6.5l1-1"/>`)
	iconRename      = icon(`<path d="M4 20h4L19.5 8.5a2.1 2.1 0 0 0-3-3L5 17z"/><path d="m14.5 5.5 3 3"/>`)
	iconMove        = icon(`<path d="M3.5 7.5A2 2 0 0 1 5.5 5.5h3.2a2 2 0 0 1 1.4.6l1 1a2 2 0 0 0 1.4.6h5.5a2 2 0 0 1 2 2v6.8a2 2 0 0 1-2 2H5.5a2 2 0 0 1-2-2z"/><path d="M12 11v6M9.5 14.5 12 17l2.5-2.5"/>`)
	iconTrash       = icon(`<path d="M4 7h16M9.5 7V5.4A1.4 1.4 0 0 1 10.9 4h2.2a1.4 1.4 0 0 1 1.4 1.4V7"/><path d="M6.5 7l.8 12a1.2 1.2 0 0 0 1.2 1.1h7a1.2 1.2 0 0 0 1.2-1.1l.8-12"/><path d="M10.5 11v6M13.5 11v6"/>`)
	iconTransfer    = icon(`<path d="M7 3.5v15M3.5 15 7 18.5 10.5 15"/><path d="M17 20.5v-15M13.5 9 17 5.5 20.5 9"/>`)
	iconLink        = icon(`<path d="M9.5 14.5l5-5"/><path d="M11 6.8 12.8 5a3.9 3.9 0 0 1 5.5 5.5l-1.8 1.8"/><path d="M13 17.2 11.2 19a3.9 3.9 0 0 1-5.5-5.5l1.8-1.8"/>`)
	iconCopy        = icon(`<rect x="9" y="9" width="11.5" height="11.5" rx="2"/><path d="M15 5.5V5a1.5 1.5 0 0 0-1.5-1.5H5.5A1.5 1.5 0 0 0 4 5v8a1.5 1.5 0 0 0 1.5 1.5H6"/>`)
	iconEye         = icon(`<path d="M2.5 12S6 5.5 12 5.5 21.5 12 21.5 12 18 18.5 12 18.5 2.5 12 2.5 12z"/><circle cx="12" cy="12" r="3"/>`)
	iconExternal    = icon(`<path d="M14.5 4H20v5.5"/><path d="M20 4l-8.5 8.5"/><path d="M18 14.5V19a1.5 1.5 0 0 1-1.5 1.5H5A1.5 1.5 0 0 1 3.5 19V7.5A1.5 1.5 0 0 1 5 6h4.5"/>`)
	iconClock       = icon(`<circle cx="12" cy="12" r="8.5"/><path d="M12 7.5V12l3 1.8"/>`)
	iconArrowLeft   = icon(`<path d="M19.5 12H5M11.5 5.5 5 12l6.5 6.5"/>`)

	iconTerminal = icon(`<rect x="3" y="4.5" width="18" height="15" rx="2.5"/><path d="m7.5 10 2.5 2.5-2.5 2.5M13 15h4"/>`)

	iconInfo  = icon(`<circle cx="12" cy="12" r="8.5"/><path d="M12 16.5V11M12 8h.01"/>`)
	iconAlert = icon(`<path d="M10.3 4.2 2.6 17.5A1.9 1.9 0 0 0 4.3 20.4h15.4a1.9 1.9 0 0 0 1.7-2.9L13.7 4.2a2 2 0 0 0-3.4 0z"/><path d="M12 9.5v4M12 17h.01"/>`)
	iconCloud = icon(`<path d="M7 18.5a4.2 4.2 0 0 1 .5-8.4 5.6 5.6 0 0 1 10.7 1.6A3.6 3.6 0 0 1 17.8 18.5z"/>`)
	iconLock  = icon(`<rect x="4.5" y="10.5" width="15" height="10" rx="2"/><path d="M8 10.5V7.5a4 4 0 0 1 8 0v3"/>`)

	// A folder, filled: the picture a folder gets in the grid, as a file
	// manager draws one. The tab along the top is what makes it read as a
	// folder rather than a card.
	iconFolderFilled = icon(`<path d="M2.2 8.4a2.2 2.2 0 0 1 2.2-2.2h3.9a2.2 2.2 0 0 1 1.7.8l.9 1.1a1.2 1.2 0 0 0 .9.4h8a2.2 2.2 0 0 1 2.2 2.2v7.9a2.2 2.2 0 0 1-2.2 2.2H4.4a2.2 2.2 0 0 1-2.2-2.2z" fill="currentColor"/><path d="M2.2 10.2h19.6" stroke="currentColor" stroke-opacity="0.28" stroke-width="1.2"/>`)
	iconFolder       = icon(`<path d="M3.5 7.5A2 2 0 0 1 5.5 5.5h3.2a2 2 0 0 1 1.4.6l1 1a2 2 0 0 0 1.4.6h5.5a2 2 0 0 1 2 2v6.8a2 2 0 0 1-2 2H5.5a2 2 0 0 1-2-2z"/>`)
)

func kindIcon(k fileKind) *ui.SVG {
	switch k {
	case kindFolder:
		return iconFolder
	case kindImage:
		return iconImage
	case kindAudio:
		return iconAudio
	case kindVideo:
		return iconVideo
	case kindArchive:
		return iconArchive
	case kindDocument:
		return iconDocument
	default:
		return iconOther
	}
}

var (
	iconImage    = icon(`<rect x="3" y="4.5" width="18" height="15" rx="2.5"/><circle cx="8.5" cy="10" r="1.6"/><path d="m4 17 4.5-4.5a1.5 1.5 0 0 1 2.1 0L15 17"/><path d="m13.5 15.5 1.7-1.7a1.5 1.5 0 0 1 2.1 0L20 16.5"/>`)
	iconAudio    = icon(`<path d="M9 17.5V8.8a1 1 0 0 1 .8-1L19 6v8.7"/><circle cx="6.5" cy="17.5" r="2.5"/><circle cx="16.5" cy="14.7" r="2.5"/>`)
	iconVideo    = icon(`<rect x="3" y="5" width="18" height="14" rx="2.5"/><path d="m10.5 9.5 5 2.5-5 2.5z"/>`)
	iconArchive  = icon(`<rect x="3" y="4.5" width="18" height="4.5" rx="1.5"/><path d="M5 9v9.5a1.5 1.5 0 0 0 1.5 1.5h11A1.5 1.5 0 0 0 19 18.5V9"/><path d="M10 13h4"/>`)
	iconDocument = icon(`<path d="M14 3.5H7A1.5 1.5 0 0 0 5.5 5v14A1.5 1.5 0 0 0 7 20.5h10a1.5 1.5 0 0 0 1.5-1.5V8z"/><path d="M14 3.5V8h4.5"/><path d="M9 13h6M9 16.5h4"/>`)
	iconOther    = icon(`<path d="M14 3.5H7A1.5 1.5 0 0 0 5.5 5v14A1.5 1.5 0 0 0 7 20.5h10a1.5 1.5 0 0 0 1.5-1.5V8z"/><path d="M14 3.5V8h4.5"/>`)
)

// iconFor names a glyph for the menus, which refer to icons by name so
// that a menu can be built from data.
func iconFor(name string) *ui.SVG {
	switch name {
	case "dashboard":
		return iconDashboard
	case "files":
		return iconFiles
	case "recent":
		return iconRecent
	case "settings":
		return iconSettings
	case "sliders":
		return iconSliders
	case "bucket":
		return iconBucket
	case "plus":
		return iconPlus
	case "close":
		return iconClose
	case "check":
		return iconCheck
	case "folder":
		return iconFolder
	case "newfolder":
		return iconNewFolder
	case "upload":
		return iconUpload
	case "download":
		return iconDownload
	case "trash":
		return iconTrash
	case "rename":
		return iconRename
	case "move":
		return iconMove
	case "share":
		return iconShare
	case "link":
		return iconLink
	case "copy":
		return iconCopy
	case "eye":
		return iconEye
	case "external":
		return iconExternal
	case "refresh":
		return iconRefresh
	case "search":
		return iconSearch
	case "grid":
		return iconGrid
	case "list":
		return iconList
	case "up":
		return iconUp
	case "more":
		return iconMore
	case "info":
		return iconInfo
	case "alert":
		return iconAlert
	case "transfer":
		return iconTransfer
	case "cloud":
		return iconCloud
	case "lock":
		return iconLock
	case "terminal":
		return iconTerminal
	case "sparkle":
		return iconSparkle
	default:
		return iconOther
	}
}
