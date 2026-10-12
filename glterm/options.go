package glterm

// Options configure Run.
type Options struct {
	// Shell and Args are the program the window runs.
	Shell string
	Args  []string
	// FontPt is the font size in points. Zero means 13.
	FontPt float64
	// Tabs are more programs a window with tabs opens beside the first.
	Tabs []Tab
}

// Tab is a program a tab runs. An empty Shell is the Options' own.
type Tab struct {
	Title string
	Shell string
	Args  []string
}
