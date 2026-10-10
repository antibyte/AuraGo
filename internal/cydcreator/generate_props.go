package cydcreator

// genPropValue returns a widget's prop or the catalog default.
func genPropValue(w *Widget, spec *WidgetSpec, name string) any {
	if v, ok := w.Props[name]; ok {
		return v
	}
	if spec != nil {
		if ps, ok := spec.prop(name); ok {
			return ps.Default
		}
	}
	return nil
}

func genPropInt(w *Widget, spec *WidgetSpec, name string) int {
	n, _ := propInt(genPropValue(w, spec, name))
	return n
}

func genPropString(w *Widget, spec *WidgetSpec, name string) string {
	s, _ := genPropValue(w, spec, name).(string)
	return s
}

func genPropBool(w *Widget, spec *WidgetSpec, name string) bool {
	b, _ := genPropValue(w, spec, name).(bool)
	return b
}

func genPropList(w *Widget, spec *WidgetSpec, name string) []string {
	l, _ := propList(genPropValue(w, spec, name))
	return l
}
