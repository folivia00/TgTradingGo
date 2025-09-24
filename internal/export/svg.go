package export

import (
	"bytes"
	"fmt"
)

type Line struct{ X, Y float64 }

type Marker struct {
	X    float64
	Y    float64
	Kind string
}

// SimpleSVGChart — очень простой генератор SVG (одна линия + маркеры)
func SimpleSVGChart(w, h int, line []Line, marks []Marker, title string) []byte {
	if w <= 0 {
		w = 900
	}
	if h <= 0 {
		h = 300
	}
	// найдём min/max
	minx, maxx := line[0].X, line[len(line)-1].X
	miny, maxy := line[0].Y, line[0].Y
	for _, p := range line {
		if p.Y < miny {
			miny = p.Y
		}
		if p.Y > maxy {
			maxy = p.Y
		}
	}
	sx := float64(w-80) / (maxx - minx + 1e-9)
	sy := float64(h-60) / (maxy - miny + 1e-9)
	var b bytes.Buffer
	fmt.Fprintf(&b, "<svg xmlns='http://www.w3.org/2000/svg' width='%d' height='%d' viewBox='0 0 %d %d'>", w, h, w, h)
	b.WriteString("<rect width='100%' height='100%' fill='#0b0f17'/>")
	b.WriteString("<g transform='translate(40,20)'>")
	// оси
	b.WriteString("<line x1='0' y1='0' x2='0' y2='")
	b.WriteString(itoa(h - 60))
	b.WriteString("' stroke='#1f2837' />")
	b.WriteString("<line x1='0' y1='")
	b.WriteString(itoa(h - 60))
	b.WriteString("' x2='")
	b.WriteString(itoa(w - 80))
	b.WriteString("' y2='")
	b.WriteString(itoa(h - 60))
	b.WriteString("' stroke='#1f2837' />")
	// линия
	b.WriteString("<polyline fill='none' stroke='#59a6ff' stroke-width='1.5' points='")
	for i, p := range line {
		x := (p.X - minx) * sx
		y := float64(h-60) - (p.Y-miny)*sy
		if i > 0 {
			b.WriteByte(' ')
		}
		fmt.Fprintf(&b, "%.2f,%.2f", x, y)
	}
	b.WriteString("'/>")
	// маркеры
	for _, m := range marks {
		x := (m.X - minx) * sx
		y := float64(h-60) - (m.Y-miny)*sy
		color := "#8bff9b"
		if m.Kind == "sell" {
			color = "#ff7a7a"
		}
		fmt.Fprintf(&b, "<circle cx='%.2f' cy='%.2f' r='3' fill='%s' />", x, y, color)
	}
	b.WriteString("</g>")
	fmt.Fprintf(&b, "<text x='16' y='18' fill='#e6edf3' font-family='Inter' font-size='14'>%s</text>", title)
	b.WriteString("</svg>")
	return b.Bytes()
}

func itoa(x int) string { return fmt.Sprintf("%d", x) }
