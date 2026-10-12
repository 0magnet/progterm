package glterm

// Options configure Run.
type Options struct {
	// Shell and Args are the program the window runs.
	Shell string
	Args  []string
	// FontPt is the font size in points. Zero means 13.
	FontPt float64
}
