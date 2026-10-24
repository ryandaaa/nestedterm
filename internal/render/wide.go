package render

// NormalizeWide repairs half-overwritten wide characters. A wide glyph is a
// Width==2 lead cell followed by a Continuation cell; a border or overlay drawn
// over only one half leaves an orphan, and terminals disagree about how to
// paint that. Orphans are replaced with blanks so every glyph is either whole
// or gone.
func (c *Canvas) NormalizeWide() {
	for y := 0; y < c.Height; y++ {
		row := c.Cells[y*c.Width : (y+1)*c.Width]
		for x := 0; x < len(row); x++ {
			cell := &row[x]
			switch {
			case cell.Continuation:
				// Valid pairs are skipped below, so reaching one means no lead.
				*cell = BlankCell()
			case cell.Width == 2:
				if x+1 < len(row) && row[x+1].Continuation {
					x++
				} else {
					*cell = BlankCell()
				}
			}
		}
	}
}
