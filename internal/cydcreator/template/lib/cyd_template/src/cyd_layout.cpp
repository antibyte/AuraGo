// cyd_layout.cpp - layout checker for the runtime's own screens (always
// compiled; the setup screen runs it after every view change). The on-device
// self-test that drives it lives in cyd_layout_selftest.cpp.
#include "cyd_private.h"

namespace cyd {
namespace {

constexpr int kMaxLogged = 5;  // warn lines per check; the rest is one summary line

struct Check {
  const char* view;
  const lv_obj_t* root;
  const lv_obj_t* overlay;
  const lv_obj_t* scroller;
  lv_area_t visible;
  bool log;
  bool placement_only;
  int problems;
};

const char* kind_of(const lv_obj_t* o) {
  if (lv_obj_check_type(o, &lv_label_class)) return "label";
  if (lv_obj_check_type(o, &lv_list_button_class)) return "list item";
  if (lv_obj_check_type(o, &lv_button_class)) return "button";
  if (lv_obj_check_type(o, &lv_textarea_class)) return "textarea";
  if (lv_obj_check_type(o, &lv_keyboard_class)) return "keyboard";
  if (lv_obj_check_type(o, &lv_list_class)) return "list";
  if (lv_obj_check_type(o, &lv_qrcode_class)) return "qrcode";
  return "obj";
}

// Widgets that manage their own children (or have none worth checking).
bool is_leaf(const lv_obj_t* o) {
  return lv_obj_check_type(o, &lv_textarea_class) || lv_obj_check_type(o, &lv_keyboard_class) ||
         lv_obj_check_type(o, &lv_qrcode_class);
}

// "<kind> "<text>" [x,y wxh]": the object's own text or its first label's,
// cut at a UTF-8 boundary.
void describe(const lv_obj_t* o, char* buf, size_t n) {
  const lv_obj_t* l = lv_obj_check_type(o, &lv_label_class) ? o : nullptr;
  for (uint32_t i = 0; l == nullptr && i < lv_obj_get_child_count(o); ++i) {
    const lv_obj_t* c = lv_obj_get_child(o, (int32_t)i);
    if (lv_obj_check_type(c, &lv_label_class)) l = c;
  }
  char text[28] = "";
  if (l != nullptr) {
    const char* s = lv_label_get_text(l);
    size_t len = s ? strlen(s) : 0;
    if (len > sizeof(text) - 1) {
      len = sizeof(text) - 1;
      while (len > 0 && ((uint8_t)s[len] & 0xC0) == 0x80) --len;
    }
    memcpy(text, s, len);
    text[len] = '\0';
  }
  lv_area_t a;
  lv_obj_get_coords(o, &a);
  snprintf(buf, n, "%s \"%s\" [%ld,%ld %ldx%ld]", kind_of(o), text, (long)a.x1, (long)a.y1,
           (long)lv_area_get_width(&a), (long)lv_area_get_height(&a));
}

// placement: the object is outside the visible area or its parent's content
// box (what a smaller element can fix); everything else is a design finding.
void report(Check& c, const lv_obj_t* o, const char* what, bool placement = false) {
  if (c.placement_only && !placement) return;
  ++c.problems;
  if (!c.log || c.problems > kMaxLogged) return;
  char d[96];
  describe(o, d, sizeof(d));
  log::warn("layout: %s %s %s", c.view, d, what);
}

bool inside(const lv_area_t& a, const lv_area_t& box, bool vertical) {
  if (a.x1 < box.x1 || a.x2 > box.x2) return false;
  return !vertical || (a.y1 >= box.y1 && a.y2 <= box.y2);
}

bool shown(const lv_obj_t* o) { return !lv_obj_has_flag(o, LV_OBJ_FLAG_HIDDEN); }

void check_obj(Check& c, lv_obj_t* o, bool in_scroller) {
  lv_area_t a;
  lv_obj_get_coords(o, &a);
  // Scroller children are checked against the scroller (horizontally) below;
  // the screen itself is the visible area plus what the overlay covers.
  bool exempt = in_scroller || o == c.overlay || (o == c.root && lv_obj_get_parent(o) == nullptr);
  if (!exempt && !inside(a, c.visible, true)) report(c, o, "is outside the visible area", true);
  if (o != c.scroller && lv_obj_get_class(o) == &lv_obj_class && lv_obj_has_flag(o, LV_OBJ_FLAG_SCROLLABLE)) {
    report(c, o, "is a scrollable container");
  }
  if (lv_obj_check_type(o, &lv_label_class) && lv_label_get_long_mode(o) == LV_LABEL_LONG_MODE_WRAP &&
      (lv_obj_get_self_width(o) > lv_obj_get_content_width(o) ||
       lv_obj_get_self_height(o) > lv_obj_get_content_height(o))) {
    report(c, o, "text does not fit");
  }
  if (is_leaf(o)) return;

  lv_area_t box = a;
  box.x1 += lv_obj_get_style_space_left(o, LV_PART_MAIN);
  box.x2 -= lv_obj_get_style_space_right(o, LV_PART_MAIN);
  box.y1 += lv_obj_get_style_space_top(o, LV_PART_MAIN);
  box.y2 -= lv_obj_get_style_space_bottom(o, LV_PART_MAIN);
  bool scrolls = o == c.scroller;
  uint32_t n = lv_obj_get_child_count(o);
  for (uint32_t i = 0; i < n; ++i) {
    lv_obj_t* ch = lv_obj_get_child(o, (int32_t)i);
    if (!shown(ch) || lv_obj_has_flag(ch, LV_OBJ_FLAG_FLOATING)) continue;
    lv_area_t ca;
    lv_obj_get_coords(ch, &ca);
    if (!inside(ca, box, !scrolls)) report(c, ch, "leaves its parent's content box", true);
    // Siblings must not overlap (absolute layouts such as the header); the
    // overlay (keyboard) is meant to cover the content.
    for (uint32_t j = i + 1; !scrolls && ch != c.overlay && j < n; ++j) {
      lv_obj_t* sib = lv_obj_get_child(o, (int32_t)j);
      if (!shown(sib) || sib == c.overlay || lv_obj_has_flag(sib, LV_OBJ_FLAG_FLOATING)) continue;
      lv_area_t sa;
      lv_obj_get_coords(sib, &sa);
      if (ca.x1 <= sa.x2 && sa.x1 <= ca.x2 && ca.y1 <= sa.y2 && sa.y1 <= ca.y2) report(c, ch, "overlaps a sibling");
    }
    check_obj(c, ch, in_scroller || scrolls);
  }
}

}  // namespace

namespace detail {

int layout_check(lv_obj_t* root, const char* view, const lv_obj_t* overlay, const lv_obj_t* scroller, bool log,
                 bool placement_only) {
  if (root == nullptr) return 0;
  lv_obj_update_layout(root);
  Check c{view, root, overlay, scroller, {0, 0, CYD_SCREEN_W - 1, CYD_SCREEN_H - 1}, log, placement_only, 0};
  if (overlay != nullptr && shown(overlay)) {
    lv_area_t oa;
    lv_obj_get_coords(overlay, &oa);
    c.visible.y2 = oa.y1 - 1;
  } else {
    c.overlay = nullptr;
  }
  if (shown(root)) check_obj(c, root, false);
  if (log && c.problems > kMaxLogged) log::warn("layout: %s %d more", view, c.problems - kMaxLogged);
  return c.problems;
}

}  // namespace detail

}  // namespace cyd
