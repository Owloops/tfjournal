package tui

import (
	"fmt"
	"image"
	"time"

	ui "github.com/gizak/termui/v3"

	"github.com/Owloops/tfjournal/run"
)

type GanttChart struct {
	ui.Block
	Resources     []run.Resource
	TotalDuration time.Duration
	BarColor      ui.Color
	PendingColor  ui.Color
	LabelStyle    ui.Style
	HeaderStyle   ui.Style
}

func NewGanttChart() *GanttChart {
	return &GanttChart{
		Block:        *ui.NewBlock(),
		BarColor:     ui.ColorGreen,
		PendingColor: ui.ColorYellow,
		LabelStyle:   ui.NewStyle(ui.ColorWhite),
		HeaderStyle:  ui.NewStyle(ui.ColorCyan, ui.ColorClear, ui.ModifierBold),
	}
}

func (g *GanttChart) Draw(buf *ui.Buffer) {
	g.Block.Draw(buf)

	if len(g.Resources) == 0 {
		buf.SetString("No resource events recorded", g.LabelStyle, image.Pt(g.Inner.Min.X, g.Inner.Min.Y))
		return
	}

	labelWidth := g.calculateLabelWidth()
	barWidth := g.Inner.Dx() - labelWidth - 3

	barWidth = max(10, barWidth)

	y := g.Inner.Min.Y

	g.drawHeader(buf, labelWidth, barWidth, y)
	y++

	g.drawSeparator(buf, labelWidth, barWidth, y)
	y++

	for _, res := range g.Resources {
		if y >= g.Inner.Max.Y {
			break
		}
		g.drawResourceRow(buf, res, labelWidth, barWidth, y)
		y++
	}
}

func (g *GanttChart) calculateLabelWidth() int {
	maxLen := 10
	for _, res := range g.Resources {
		if len(res.Address) > maxLen {
			maxLen = len(res.Address)
		}
	}
	availableWidth := g.Inner.Dx() / 2
	if maxLen > availableWidth {
		maxLen = availableWidth
	}
	return maxLen
}

func (g *GanttChart) drawHeader(buf *ui.Buffer, labelWidth, barWidth, y int) {
	label := "Resource"
	buf.SetString(label, g.HeaderStyle, image.Pt(g.Inner.Min.X, y))

	barStartX := g.Inner.Min.X + labelWidth + 3
	maxX := g.Inner.Max.X

	buf.SetString("0s", g.HeaderStyle, image.Pt(barStartX, y))

	endStr := formatDuration(g.TotalDuration)
	endX := min(barStartX+barWidth-len(endStr), maxX-len(endStr))

	ticks := g.calculateTicks(barWidth)
	for _, tick := range ticks {
		pos := int(float64(tick) / float64(g.TotalDuration) * float64(barWidth))
		tickStr := formatDuration(tick)
		tickX := barStartX + pos - len(tickStr)/2
		if tickX > barStartX+2 && tickX+len(tickStr) < endX-1 {
			buf.SetString(tickStr, g.HeaderStyle, image.Pt(tickX, y))
		}
	}

	buf.SetString(endStr, g.HeaderStyle, image.Pt(endX, y))
}

func (g *GanttChart) calculateTicks(barWidth int) []time.Duration {
	totalSec := g.TotalDuration.Seconds()
	if totalSec <= 0 {
		return nil
	}

	targetTicks := max(2, barWidth/15)
	intervals := []float64{1, 2, 5, 10, 15, 30, 60, 120, 300, 600}
	var interval float64
	for _, iv := range intervals {
		if totalSec/iv <= float64(targetTicks) {
			interval = iv
			break
		}
	}
	if interval == 0 {
		interval = totalSec / float64(targetTicks)
	}

	var ticks []time.Duration
	for t := interval; t < totalSec*0.9; t += interval {
		ticks = append(ticks, time.Duration(t*float64(time.Second)))
	}
	return ticks
}

func (g *GanttChart) drawSeparator(buf *ui.Buffer, labelWidth, barWidth, y int) {
	for i := range labelWidth {
		buf.SetCell(ui.NewCell('─', g.LabelStyle), image.Pt(g.Inner.Min.X+i, y))
	}
	buf.SetCell(ui.NewCell('┼', g.LabelStyle), image.Pt(g.Inner.Min.X+labelWidth+1, y))
	for i := range barWidth {
		buf.SetCell(ui.NewCell('─', g.LabelStyle), image.Pt(g.Inner.Min.X+labelWidth+3+i, y))
	}
}

func (g *GanttChart) drawResourceRow(buf *ui.Buffer, res run.Resource, labelWidth, barWidth, y int) {
	label := res.Address
	if len(label) > labelWidth {
		label = label[:labelWidth-3] + "..."
	}
	buf.SetString(label, g.LabelStyle, image.Pt(g.Inner.Min.X, y))

	buf.SetCell(ui.NewCell('│', g.LabelStyle), image.Pt(g.Inner.Min.X+labelWidth+1, y))

	if res.StartTime.IsZero() || g.TotalDuration == 0 {
		return
	}

	startOffset := res.StartTime.Sub(time.UnixMilli(0))
	startPos := int(float64(startOffset) / float64(g.TotalDuration) * float64(barWidth-1))
	startPos = max(0, startPos)
	if startPos >= barWidth-1 {
		startPos = barWidth - 2
	}

	var endPos int
	var barColor ui.Color
	if res.EndTime.IsZero() {
		endPos = barWidth - 1
		barColor = g.PendingColor
	} else {
		endOffset := res.EndTime.Sub(time.UnixMilli(0))
		endPos = int(float64(endOffset) / float64(g.TotalDuration) * float64(barWidth-1))
		endPos = max(startPos+1, endPos)
		endPos = min(barWidth-1, endPos)
		barColor = g.BarColor
	}

	barStartX := g.Inner.Min.X + labelWidth + 3 + startPos
	for i := startPos; i < endPos; i++ {
		buf.SetCell(ui.NewCell('█', ui.NewStyle(barColor)), image.Pt(barStartX+i-startPos, y))
	}
}

func formatDuration(d time.Duration) string {
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
	if d < time.Minute {
		return fmt.Sprintf("%.0fs", d.Seconds())
	}
	if d < time.Hour {
		mins := int(d.Minutes())
		secs := int(d.Seconds()) % 60
		if secs > 0 {
			return fmt.Sprintf("%dm%ds", mins, secs)
		}
		return fmt.Sprintf("%dm", mins)
	}
	return d.Round(time.Second).String()
}

func (g *GanttChart) SetData(r *run.Run) {
	g.Resources = r.Resources

	if len(r.Resources) == 0 {
		g.TotalDuration = r.Duration()
		return
	}

	var maxEndMs int64
	for _, res := range r.Resources {
		endMs := res.StartTime.UnixMilli() + res.DurationMs
		if endMs > maxEndMs {
			maxEndMs = endMs
		}
	}

	if maxEndMs == 0 {
		maxEndMs = r.DurationMs
	}
	if maxEndMs == 0 {
		maxEndMs = 1000
	}

	g.TotalDuration = time.Duration(maxEndMs) * time.Millisecond
}
