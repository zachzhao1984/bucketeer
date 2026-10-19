package main

import "github.com/egoist/mygo/ui"

// The icons the interface draws, as one set: 24-unit strokes on a 24-unit
// box, with subtle tint fills for visual depth and instant file type recognition.

func icon(shapes string) *ui.SVG {
	return ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round">` + shapes + `</svg>`))
}

var (
	iconDashboard = icon(`<rect x="3" y="3" width="7" height="9" rx="1.5"/><rect x="14" y="3" width="7" height="5" rx="1.5"/><rect x="14" y="12" width="7" height="9" rx="1.5"/><rect x="3" y="16" width="7" height="5" rx="1.5"/>`)
	iconFiles     = icon(`<path d="M3 7a2 2 0 0 1 2-2h4l2 2h8a2 2 0 0 1 2 2v8a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2z"/>`)
	iconSettings  = icon(`<path d="M4 7h9M19 7h1M4 17h5M15 17h5"/><circle cx="16" cy="7" r="2.5"/><circle cx="12" cy="17" r="2.5"/>`)
	iconBucket    = icon(`<ellipse cx="12" cy="6" rx="8" ry="3" fill="currentColor" fill-opacity="0.15"/><path d="M4 6v6c0 1.7 3.6 3 8 3s8-1.3 8-3V6"/><path d="M4 12v6c0 1.7 3.6 3 8 3s8-1.3 8-3v-6"/>`)

	iconUpload   = icon(`<path d="M12 16V4M8 8l4-4 4 4"/><path d="M4 16v2a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2v-2"/>`)
	iconDownload = icon(`<path d="M12 4v12M8 12l4 4 4-4"/><path d="M4 16v2a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2v-2"/>`)
	iconPlus     = icon(`<path d="M12 5v14M5 12h14"/>`)
	iconRefresh  = icon(`<path d="M20 11a8 8 0 1 0-.6 4"/><path d="M20 4v7h-7"/>`)
	iconClose    = icon(`<path d="M6 6l12 12M18 6L6 18"/>`)
	iconCheck    = icon(`<path d="M20 6 9 17l-5-5"/>`)

	iconGrid   = icon(`<rect x="3" y="3" width="7" height="7" rx="1.5"/><rect x="14" y="3" width="7" height="7" rx="1.5"/><rect x="3" y="14" width="7" height="7" rx="1.5"/><rect x="14" y="14" width="7" height="7" rx="1.5"/>`)
	iconList   = icon(`<path d="M9 6h11M9 12h11M9 18h11"/><circle cx="4.5" cy="6" r="1.3" fill="currentColor"/><circle cx="4.5" cy="12" r="1.3" fill="currentColor"/><circle cx="4.5" cy="18" r="1.3" fill="currentColor"/>`)
	iconSearch = icon(`<circle cx="11" cy="11" r="7"/><path d="m20 20-3.6-3.6"/>`)
	iconMore   = icon(`<circle cx="5" cy="12" r="1.6" fill="currentColor"/><circle cx="12" cy="12" r="1.6" fill="currentColor"/><circle cx="19" cy="12" r="1.6" fill="currentColor"/>`)

	iconUp          = icon(`<path d="M12 19V5M5 12l7-7 7 7"/>`)
	iconChevron     = icon(`<path d="m9 6 6 6-6 6"/>`)
	iconChevronDown = icon(`<path d="m6 9 6 6 6-6"/>`)
	iconNewFile     = icon(`<path d="M3 7a2 2 0 0 1 2-2h4l2 2h8a2 2 0 0 1 2 2v8a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2z" fill="currentColor" fill-opacity="0.12"/><path d="M12 11.5v5M9.5 14h5"/>`)
	iconShare       = icon(`<path d="M10 13.5a5 5 0 0 0 7 0l3-3a5 5 0 0 0-7-7l-1 1"/><path d="M14 10.5a5 5 0 0 0-7 0l-3 3a5 5 0 0 0 7 7l1-1"/>`)
	iconRename      = icon(`<path d="M4 20h4L20 8l-4-4L4 16z"/><path d="M14.5 5.5 18.5 9.5"/>`)
	iconMove        = icon(`<path d="M3 7a2 2 0 0 1 2-2h4l2 2h8a2 2 0 0 1 2 2v8a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2z"/><path d="M12 11v6M9.5 14.5 12 17l2.5-2.5"/>`)
	iconTrash       = icon(`<path d="M4 7h16M9.5 7V5.2A1.2 1.2 0 0 1 10.7 4h2.6a1.2 1.2 0 0 1 1.2 1.2V7"/><path d="M6.5 7l.9 12a1 1 0 0 0 1 1h7.2a1 1 0 0 0 1-1l.9-12"/><path d="M10.5 11v6M13.5 11v6"/>`)
	iconTransfer    = icon(`<path d="M7 3v15M3.5 14.5 7 18l3.5-3.5"/><path d="M17 21V6M13.5 9.5 17 6l3.5 3.5"/>`)
	iconLink        = icon(`<path d="M9 15l6-6"/><path d="M11 6.5 13 4.5a4.2 4.2 0 0 1 6 6L17 12.5"/><path d="M13 17.5 11 19.5a4.2 4.2 0 0 1-6-6L7 11.5"/>`)
	iconCopy        = icon(`<rect x="9" y="9" width="11.5" height="11.5" rx="2"/><path d="M15 5.5V5a1.5 1.5 0 0 0-1.5-1.5H5.5A1.5 1.5 0 0 0 4 5v8a1.5 1.5 0 0 0 1.5 1.5H6"/>`)
	iconEye         = icon(`<path d="M2.5 12S6 5.5 12 5.5 21.5 12 21.5 12 18 18.5 12 18.5 2.5 12 2.5 12z"/><circle cx="12" cy="12" r="3"/>`)
	iconExternal    = icon(`<path d="M14.5 4H20v5.5"/><path d="M20 4l-8.5 8.5"/><path d="M18 14.5V19a1.5 1.5 0 0 1-1.5 1.5H5A1.5 1.5 0 0 1 3.5 19V7.5A1.5 1.5 0 0 1 5 6h4.5"/>`)

	iconInfo   = icon(`<circle cx="12" cy="12" r="8.5"/><path d="M12 16.5V11M12 8h.01"/>`)
	iconAlert  = icon(`<path d="M10.3 3.9 1.9 18a2 2 0 0 0 1.7 3h16.8a2 2 0 0 0 1.7-3L13.7 3.9a2 2 0 0 0-3.4 0z"/><path d="M12 9.5v4M12 17h.01"/>`)
	iconCloud  = icon(`<path d="M7 18.5a4.2 4.2 0 0 1 .5-8.4 5.6 5.6 0 0 1 10.7 1.6A3.6 3.6 0 0 1 17.8 18.5z"/>`)
	iconFolder = icon(`<path d="M3 7a2 2 0 0 1 2-2h4l2 2h8a2 2 0 0 1 2 2v8a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2z" fill="currentColor" fill-opacity="0.18"/>`)
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
	iconImage    = icon(`<rect x="3" y="4.5" width="18" height="15" rx="2.5" fill="currentColor" fill-opacity="0.12"/><circle cx="8.5" cy="10" r="1.6"/><path d="m4 17 4.5-4.5a1.5 1.5 0 0 1 2.1 0L15 17"/><path d="m13.5 15.5 1.7-1.7a1.5 1.5 0 0 1 2.1 0L20 16.5"/>`)
	iconAudio    = icon(`<path d="M9 17.5V8.8a1 1 0 0 1 .8-1L19 6v8.7"/><circle cx="6.5" cy="17.5" r="2.5" fill="currentColor" fill-opacity="0.25"/><circle cx="16.5" cy="14.7" r="2.5" fill="currentColor" fill-opacity="0.25"/>`)
	iconVideo    = icon(`<rect x="3" y="5" width="18" height="14" rx="2.5" fill="currentColor" fill-opacity="0.12"/><path d="m10.5 9.5 5 2.5-5 2.5z" fill="currentColor" fill-opacity="0.3"/>`)
	iconArchive  = icon(`<rect x="3" y="4.5" width="18" height="4.5" rx="1.5" fill="currentColor" fill-opacity="0.25"/><path d="M5 9v9.5a1.5 1.5 0 0 0 1.5 1.5h11A1.5 1.5 0 0 0 19 18.5V9" fill="currentColor" fill-opacity="0.12"/><path d="M10 13h4"/>`)
	iconDocument = icon(`<path d="M14 3.5H7A1.5 1.5 0 0 0 5.5 5v14A1.5 1.5 0 0 0 7 20.5h10a1.5 1.5 0 0 0 1.5-1.5V8z" fill="currentColor" fill-opacity="0.12"/><path d="M14 3.5V8h4.5"/><path d="M9 13h6M9 16.5h4"/>`)
	iconOther    = icon(`<path d="M14 3.5H7A1.5 1.5 0 0 0 5.5 5v14A1.5 1.5 0 0 0 7 20.5h10a1.5 1.5 0 0 0 1.5-1.5V8z" fill="currentColor" fill-opacity="0.08"/><path d="M14 3.5V8h4.5"/>`)
)
