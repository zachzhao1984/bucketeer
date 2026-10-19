package main

// The type filter and the sort field: the two lists the files toolbar
// drops, and the labels they show while they are closed.

var typeFilters = []struct {
	Label string
	Kind  string
}{
	{"All types", "all"},
	{"Folders", "folders"},
	{"Images", "image"},
	{"Video", "video"},
	{"Audio", "audio"},
	{"Documents", "document"},
	{"Archives", "archive"},
	{"Other", "other"},
}

func (a *App) filterLabel() string {
	for _, f := range typeFilters {
		if f.Kind == a.filter {
			return f.Label
		}
	}
	return typeFilters[0].Label
}

var sortOptions = []struct {
	Label string
	Field SortField
}{
	{"Name", sortName},
	{"Size", sortSize},
	{"Date modified", sortModified},
}

func (a *App) sortLabel() string {
	for _, o := range sortOptions {
		if o.Field == a.sort {
			return o.Label
		}
	}
	return sortOptions[0].Label
}
