package virtualcomputers

type DesktopActionRequest struct {
	Operation    string  `json:"operation"`
	ControlToken string  `json:"control_token,omitempty"`
	App          string  `json:"app,omitempty"`
	WindowID     string  `json:"window_id,omitempty"`
	ElementRef   string  `json:"element_ref,omitempty"`
	Root         string  `json:"root,omitempty"`
	Surface      string  `json:"surface,omitempty"`
	Role         string  `json:"role,omitempty"`
	Name         string  `json:"name,omitempty"`
	Text         string  `json:"text,omitempty"`
	Value        string  `json:"value,omitempty"`
	Key          string  `json:"key,omitempty"`
	Predicate    string  `json:"predicate,omitempty"`
	TimeoutMS    int     `json:"timeout_ms,omitempty"`
	MaxDepth     int     `json:"max_depth,omitempty"`
	ImageRef     string  `json:"image_ref,omitempty"`
	Width        int     `json:"width,omitempty"`
	Height       int     `json:"height,omitempty"`
	X            float64 `json:"x,omitempty"`
	Y            float64 `json:"y,omitempty"`
	ToX          float64 `json:"to_x,omitempty"`
	ToY          float64 `json:"to_y,omitempty"`
	DeltaX       float64 `json:"delta_x,omitempty"`
	DeltaY       float64 `json:"delta_y,omitempty"`
}

var DesktopOperations = []string{"status", "list_apps", "list_windows", "list_surfaces", "snapshot", "find", "wait", "screenshot", "launch", "close_app", "focus_window", "click", "double_click", "right_click", "type", "set_value", "select", "toggle", "expand", "collapse", "scroll_to", "press", "click_xy", "scroll", "drag"}

func DesktopInput(op string) bool {
	switch op {
	case "status", "list_apps", "list_windows", "list_surfaces", "snapshot", "find", "wait", "screenshot":
		return false
	}
	return true
}
