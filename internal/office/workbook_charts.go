package office

import (
	"encoding/xml"
	"fmt"
	"path"
	"reflect"
	"strings"

	"github.com/xuri/excelize/v2"
)

type ooxmlChartTitle struct {
	Runs []struct {
		Text string `xml:"t"`
	} `xml:"tx>rich>p>r"`
}

func (title ooxmlChartTitle) text() string {
	var out strings.Builder
	for _, run := range title.Runs {
		out.WriteString(run.Text)
	}
	return out.String()
}

type ooxmlChartAxis struct {
	Title    ooxmlChartTitle `xml:"title"`
	Position struct {
		Value string `xml:"val,attr"`
	} `xml:"axPos"`
}
type ooxmlChartSeries struct {
	MarkerColor struct {
		Value string `xml:"val,attr"`
	} `xml:"marker>spPr>solidFill>srgbClr"`
	Points []struct {
		Index struct {
			Value int `xml:"val,attr"`
		} `xml:"idx"`
		Color struct {
			Value string `xml:"val,attr"`
		} `xml:"spPr>solidFill>srgbClr"`
	} `xml:"dPt"`
	Color struct {
		Value string `xml:"val,attr"`
	} `xml:"spPr>solidFill>srgbClr"`
	LineColor struct {
		Value string `xml:"val,attr"`
	} `xml:"spPr>ln>solidFill>srgbClr"`
	Name struct {
		Formula string `xml:"strRef>f"`
		Value   string `xml:"v"`
	} `xml:"tx"`
	Categories struct {
		String string `xml:"strRef>f"`
		Number string `xml:"numRef>f"`
	} `xml:"cat"`
	X      string `xml:"xVal>numRef>f"`
	XText  string `xml:"xVal>strRef>f"`
	Y      string `xml:"yVal>numRef>f"`
	Values string `xml:"val>numRef>f"`
}
type ooxmlChartPlot struct {
	Grouping struct {
		Value string `xml:"val,attr"`
	} `xml:"grouping"`
	Series    []ooxmlChartSeries `xml:"ser"`
	Direction struct {
		Value string `xml:"val,attr"`
	} `xml:"barDir"`
}

func chartTexts(data []byte) string {
	var value struct {
		Paragraphs []struct {
			Runs []struct {
				Text string `xml:"t"`
			} `xml:"r"`
		} `xml:"chart>title>tx>rich>p"`
	}
	_ = xml.Unmarshal(data, &value)
	var text strings.Builder
	for _, p := range value.Paragraphs {
		for _, r := range p.Runs {
			text.WriteString(r.Text)
		}
	}
	return text.String()
}
func readEditorCharts(parts map[string][]byte, book EditorWorkbook) []EditorChart {
	charts := []EditorChart{}
	for _, id := range book.SheetOrder {
		sheet := book.Sheets[id]
		sheetPath := sheetPartByName(parts, sheet.Name)
		var sheetXML struct {
			Drawing struct {
				ID string `xml:"id,attr"`
			} `xml:"drawing"`
		}
		if xml.Unmarshal(parts[sheetPath], &sheetXML) != nil || sheetXML.Drawing.ID == "" {
			continue
		}
		sheetRels := officeRelationships(parts[path.Join(path.Dir(sheetPath), "_rels", path.Base(sheetPath)+".rels")])
		drawingPath := resolveOfficePart(sheetPath, sheetRels[sheetXML.Drawing.ID])
		var drawing struct {
			Anchors []struct {
				From struct {
					Col    int `xml:"col"`
					Row    int `xml:"row"`
					ColOff int `xml:"colOff"`
					RowOff int `xml:"rowOff"`
				} `xml:"from"`
				To struct {
					Col int `xml:"col"`
					Row int `xml:"row"`
				} `xml:"to"`
				Size struct {
					CX int `xml:"cx,attr"`
					CY int `xml:"cy,attr"`
				} `xml:"ext"`
				Frame struct {
					Chart struct {
						ID string `xml:"id,attr"`
					} `xml:"graphic>graphicData>chart"`
					Size struct {
						CX int `xml:"cx,attr"`
						CY int `xml:"cy,attr"`
					} `xml:"xfrm>ext"`
				} `xml:"graphicFrame"`
			} `xml:",any"`
		}
		if xml.Unmarshal(parts[drawingPath], &drawing) != nil {
			continue
		}
		drawingRels := officeRelationships(parts[path.Join(path.Dir(drawingPath), "_rels", path.Base(drawingPath)+".rels")])
		for _, anchor := range drawing.Anchors {
			if anchor.Frame.Chart.ID == "" {
				continue
			}
			chartPath := resolveOfficePart(drawingPath, drawingRels[anchor.Frame.Chart.ID])
			data := parts[chartPath]
			var chart struct {
				Plot struct {
					Bar        *ooxmlChartPlot  `xml:"barChart"`
					Line       *ooxmlChartPlot  `xml:"lineChart"`
					Pie        *ooxmlChartPlot  `xml:"pieChart"`
					Scatter    *ooxmlChartPlot  `xml:"scatterChart"`
					Categories []ooxmlChartAxis `xml:"catAx"`
					Values     []ooxmlChartAxis `xml:"valAx"`
				} `xml:"chart>plotArea"`
				Legend *struct{} `xml:"chart>legend"`
			}
			if xml.Unmarshal(data, &chart) != nil {
				continue
			}
			c := EditorChart{ID: chartPath, Sheet: id, Title: chartTexts(data), Legend: chart.Legend != nil, Width: 480, Height: 290, Colors: []string{"#3979d5", "#e4a24a", "#57a78b", "#9471ce"}}
			for _, axis := range append(chart.Plot.Categories, chart.Plot.Values...) {
				if axis.Position.Value == "b" || axis.Position.Value == "t" {
					c.XTitle = axis.Title.text()
				} else {
					c.YTitle = axis.Title.text()
				}
			}
			c.Anchor, _ = excelize.CoordinatesToCellName(anchor.From.Col+1, anchor.From.Row+1)
			c.X = anchor.From.ColOff / 9525
			c.Y = anchor.From.RowOff / 9525
			if anchor.Size.CX > 0 {
				c.Width = anchor.Size.CX / 9525
				c.Height = anchor.Size.CY / 9525
			} else if anchor.To.Col > anchor.From.Col {
				c.Width = (anchor.To.Col - anchor.From.Col) * int(sheet.DefaultColumnWidth)
				c.Height = max(120, (anchor.To.Row-anchor.From.Row)*int(sheet.DefaultRowHeight))
			}
			count := 0
			for _, p := range []*ooxmlChartPlot{chart.Plot.Bar, chart.Plot.Line, chart.Plot.Pie, chart.Plot.Scatter} {
				if p != nil {
					count++
				}
			}
			var plot *ooxmlChartPlot
			switch {
			case chart.Plot.Bar != nil:
				plot = chart.Plot.Bar
				c.Type = "column"
				if plot.Direction.Value == "bar" {
					c.Type = "bar"
				}
			case chart.Plot.Line != nil:
				plot = chart.Plot.Line
				c.Type = "line"
			case chart.Plot.Pie != nil:
				plot = chart.Plot.Pie
				c.Type = "pie"
			case chart.Plot.Scatter != nil:
				plot = chart.Plot.Scatter
				c.Type = "scatter"
			default:
				c.Readonly = true
				c.Type = "unsupported"
			}
			if count > 1 || len(chart.Plot.Categories)+len(chart.Plot.Values) > 2 || plot != nil && (plot.Grouping.Value == "stacked" || plot.Grouping.Value == "percentStacked") || strings.Contains(string(data), ":trendline") {
				c.Readonly = true
				c.Type = "unsupported"
				plot = nil
			}
			if plot != nil {
				for index, s := range plot.Series {
					for _, point := range s.Points {
						if point.Color.Value != "" && point.Index.Value >= 0 && point.Index.Value < 10000 {
							for len(c.Colors) <= point.Index.Value {
								c.Colors = append(c.Colors, "#3979d5")
							}
							c.Colors[point.Index.Value] = "#" + point.Color.Value
						}
					}
					color := s.Color.Value
					if color == "" {
						color = s.MarkerColor.Value
					}
					if color == "" {
						color = s.LineColor.Value
					}
					if color != "" {
						for len(c.Colors) <= index {
							c.Colors = append(c.Colors, "#3979d5")
						}
						c.Colors[index] = "#" + color
					}
					name := s.Name.Formula
					if name == "" {
						name = s.Name.Value
					}
					cat := s.Categories.String
					if cat == "" {
						cat = s.Categories.Number
					}
					values := s.Values
					if c.Type == "scatter" {
						cat = s.X
						if cat == "" {
							cat = s.XText
						}
						values = s.Y
					}
					c.Series = append(c.Series, EditorChartSeries{Name: name, Categories: cat, Values: values, Color: color})
				}
			}
			charts = append(charts, c)
		}
	}
	return charts
}
func applyEditorCharts(f *excelize.File, before WorkbookEditorDocument, patch WorkbookEditorPatch) error {
	if len(patch.Charts) > 256 {
		return fmt.Errorf("Too many charts")
	}
	previous := map[string]EditorChart{}
	next := map[string]EditorChart{}
	for _, c := range before.Charts {
		previous[c.ID] = c
	}
	for _, c := range patch.Charts {
		if c.ID == "" || len(c.ID) > 256 || next[c.ID].ID != "" {
			return fmt.Errorf("Invalid chart identity")
		}
		next[c.ID] = c
		if _, ok := patch.Workbook.Sheets[c.Sheet]; !ok {
			return fmt.Errorf("Invalid chart sheet")
		}
		if c.Width < 120 || c.Width > 4000 || c.Height < 100 || c.Height > 4000 || len(c.Title) > 512 || len(c.Series) > 128 || len(c.Range) > 512 {
			return fmt.Errorf("Chart exceeds limits")
		}
		if _, _, err := excelize.CellNameToCoordinates(c.Anchor); err != nil {
			return err
		}
	}
	for _, old := range before.Charts {
		current, exists := next[old.ID]
		if exists && reflect.DeepEqual(old, current) {
			continue
		}
		if old.Readonly {
			return fmt.Errorf("This chart is preserved but cannot be edited")
		}
		sheet := patch.Workbook.Sheets[old.Sheet]
		if sheet != nil {
			if err := f.DeleteChart(sheet.Name, old.Anchor); err != nil {
				return err
			}
		}
	}
	types := map[string]excelize.ChartType{"column": excelize.Col, "bar": excelize.Bar, "line": excelize.Line, "pie": excelize.Pie, "scatter": excelize.Scatter}
	for _, c := range patch.Charts {
		if old, ok := previous[c.ID]; ok && reflect.DeepEqual(old, c) {
			continue
		}
		typ, ok := types[c.Type]
		if !ok {
			return fmt.Errorf("Unsupported chart type")
		}
		sheet := patch.Workbook.Sheets[c.Sheet]
		series := c.Series
		if c.Range != "" {
			segments := strings.Split(c.Range, ":")
			if len(segments) != 2 {
				return fmt.Errorf("Chart needs a rectangular range")
			}
			col, row, err := excelize.CellNameToCoordinates(segments[0])
			if err != nil {
				return err
			}
			lastCol, lastRow, err := excelize.CellNameToCoordinates(segments[1])
			if err != nil {
				return err
			}
			if col >= lastCol || row >= lastRow || lastCol-col > 127 || lastRow-row > 100000 {
				return fmt.Errorf("Chart range must include headings and at least two columns")
			}
			prefix := "'" + strings.ReplaceAll(sheet.Name, "'", "''") + "'!"
			categoryStart, _ := excelize.CoordinatesToCellName(col, row+1, true)
			categoryEnd, _ := excelize.CoordinatesToCellName(col, lastRow, true)
			series = nil
			for column := col + 1; column <= lastCol; column++ {
				name, _ := excelize.CoordinatesToCellName(column, row, true)
				start, _ := excelize.CoordinatesToCellName(column, row+1, true)
				end, _ := excelize.CoordinatesToCellName(column, lastRow, true)
				series = append(series, EditorChartSeries{Name: prefix + name, Categories: prefix + categoryStart + ":" + categoryEnd, Values: prefix + start + ":" + end})
				if typ == excelize.Pie {
					break
				}
			}
		}
		if len(series) == 0 {
			return fmt.Errorf("Chart has no data series")
		}
		chart := &excelize.Chart{Type: typ, Title: excelize.ChartTitle{Paragraph: []excelize.RichTextRun{{Text: c.Title}}}, Dimension: excelize.ChartDimension{Width: uint(c.Width), Height: uint(c.Height)}, Format: excelize.GraphicOptions{OffsetX: c.X, OffsetY: c.Y, Positioning: "oneCell"}, Legend: excelize.ChartLegend{Position: "bottom"}, XAxis: excelize.ChartAxis{Title: excelize.ChartTitle{Paragraph: []excelize.RichTextRun{{Text: c.XTitle}}}}, YAxis: excelize.ChartAxis{Title: excelize.ChartTitle{Paragraph: []excelize.RichTextRun{{Text: c.YTitle}}}}}
		if !c.Legend {
			chart.Legend.Position = "none"
		}
		for i, s := range series {
			color := s.Color
			if len(c.Colors) > 0 {
				color = c.Colors[i%len(c.Colors)]
			}
			item := excelize.ChartSeries{Name: s.Name, Categories: s.Categories, Values: s.Values}
			if color != "" {
				item.Fill = excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{strings.TrimPrefix(color, "#")}}
				item.Line.Fill = excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{strings.TrimPrefix(color, "#")}}
				item.Marker.Fill = item.Fill
			}
			if typ == excelize.Pie && len(c.Colors) > 0 {
				ref := s.Values
				if at := strings.LastIndex(ref, "!"); at >= 0 {
					ref = ref[at+1:]
				}
				ranges := parseEditorRanges(ref)
				if len(ranges) == 1 {
					for point := 0; point <= ranges[0].EndRow-ranges[0].StartRow && point < 10000; point++ {
						item.DataPoint = append(item.DataPoint, excelize.ChartDataPoint{Index: point, Fill: excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{strings.TrimPrefix(c.Colors[point%len(c.Colors)], "#")}}})
					}
				}
			}
			chart.Series = append(chart.Series, item)
		}
		if err := f.AddChart(sheet.Name, c.Anchor, chart); err != nil {
			return err
		}
	}
	return nil
}
