// cyd_wm_strings.h - WiFiManager page strings in the project language.
//
// Selected by the build flag -DWM_STRINGS_FILE=\"cyd_wm_strings.h\" (generated
// platformio.ini). Every user-visible string of the pages the portal shows
// (menu: wifi + exit) is renamed away before the English file is included and
// then redefined from cyd_strings.h; all other English definitions stay.
#ifndef CYD_WM_STRINGS_H
#define CYD_WM_STRINGS_H

#include "cyd_strings.h"

#define WM_LANGUAGE WM_LANGUAGE__en
#define HTTP_HEAD_START HTTP_HEAD_START__en
#define HTTP_PORTAL_MENU HTTP_PORTAL_MENU__en
#define HTTP_FORM_WIFI HTTP_FORM_WIFI__en
#define HTTP_FORM_END HTTP_FORM_END__en
#define HTTP_SCAN_LINK HTTP_SCAN_LINK__en
#define HTTP_SAVED HTTP_SAVED__en
#define HTTP_BACKBTN HTTP_BACKBTN__en
#define HTTP_STATUS_ON HTTP_STATUS_ON__en
#define HTTP_STATUS_OFF HTTP_STATUS_OFF__en
#define HTTP_STATUS_OFFPW HTTP_STATUS_OFFPW__en
#define HTTP_STATUS_OFFNOAP HTTP_STATUS_OFFNOAP__en
#define HTTP_STATUS_OFFFAIL HTTP_STATUS_OFFFAIL__en
#define HTTP_STATUS_NONE HTTP_STATUS_NONE__en
#define S_titlewifisaved S_titlewifisaved__en
#define S_titlewifi S_titlewifi__en
#define S_titleexit S_titleexit__en
#define S_titleclose S_titleclose__en
#define S_nonetworks S_nonetworks__en
#define S_exiting S_exiting__en
#define S_closing S_closing__en

#include "wm_strings_en.h"

#undef WM_LANGUAGE
#undef HTTP_HEAD_START
#undef HTTP_PORTAL_MENU
#undef HTTP_FORM_WIFI
#undef HTTP_FORM_END
#undef HTTP_SCAN_LINK
#undef HTTP_SAVED
#undef HTTP_BACKBTN
#undef HTTP_STATUS_ON
#undef HTTP_STATUS_OFF
#undef HTTP_STATUS_OFFPW
#undef HTTP_STATUS_OFFNOAP
#undef HTTP_STATUS_OFFFAIL
#undef HTTP_STATUS_NONE
#undef S_titlewifisaved
#undef S_titlewifi
#undef S_titleexit
#undef S_titleclose
#undef S_nonetworks
#undef S_exiting
#undef S_closing

const char WM_LANGUAGE[] PROGMEM = CYD_LANG;

const char HTTP_HEAD_START[] PROGMEM = "<!DOCTYPE html>"
"<html lang='" CYD_LANG "'><head>"
"<meta name='format-detection' content='telephone=no'>"
"<meta charset='UTF-8'>"
"<meta  name='viewport' content='width=device-width,initial-scale=1,user-scalable=no'/>"
"<title>{v}</title>";

const char * const HTTP_PORTAL_MENU[] PROGMEM = {
"<form action='/wifi'    method='get'><button>" CYD_S_PORTAL_CONFIGURE "</button></form><br/>\n", // MENU_WIFI
"<form action='/0wifi'   method='get'><button>" CYD_S_PORTAL_CONFIGURE "</button></form><br/>\n", // MENU_WIFINOSCAN
"<form action='/info'    method='get'><button>Info</button></form><br/>\n", // MENU_INFO
"<form action='/param'   method='get'><button>Setup</button></form><br/>\n", // MENU_PARAM
"<form action='/close'   method='get'><button>" CYD_S_WIFI_CLOSE "</button></form><br/>\n", // MENU_CLOSE
"<form action='/restart' method='get'><button>Restart</button></form><br/>\n", // MENU_RESTART
"<form action='/exit'    method='get'><button>" CYD_S_PORTAL_EXIT "</button></form><br/>\n", // MENU_EXIT
"<form action='/erase'   method='get'><button class='D'>Erase</button></form><br/>\n", // MENU_ERASE
"<form action='/update'  method='get'><button>Update</button></form><br/>\n", // MENU_UPDATE
"<hr><br/>" // MENU_SEP
};

const char HTTP_FORM_WIFI[] PROGMEM = "<label for='s'>" CYD_S_WIFI_SSID "</label>"
"<input id='s' name='s' maxlength='32' autocorrect='off' autocapitalize='none' placeholder='{v}'><br/>"
"<label for='p'>" CYD_S_WIFI_PASSWORD "</label>"
"<input id='p' name='p' maxlength='64' type='password' placeholder='{p}'>"
"<input type='checkbox' id='showpass' onclick='f()'> <label for='showpass'>" CYD_S_PORTAL_SHOW_PASSWORD "</label><br/>";
const char HTTP_FORM_END[] PROGMEM = "<br/><br/><button type='submit'>" CYD_S_PORTAL_SAVE "</button></form>";
const char HTTP_SCAN_LINK[] PROGMEM = "<br/><form action='/wifi?refresh=1' method='POST'><button name='refresh' value='1'>" CYD_S_PORTAL_REFRESH "</button></form>";
const char HTTP_SAVED[] PROGMEM = "<div class='msg'>" CYD_S_PORTAL_SAVING "</div>";
const char HTTP_BACKBTN[] PROGMEM = "<hr><br/><form action='/' method='get'><button>" CYD_S_WIFI_BACK "</button></form>";
const char HTTP_STATUS_ON[] PROGMEM = "<div class='msg S'><strong>" CYD_S_WIFI_CONNECTED "</strong>: {v}<br/><em><small>IP {i}</small></em></div>";
const char HTTP_STATUS_OFF[] PROGMEM = "<div class='msg {c}'><strong>" CYD_S_PORTAL_NOT_CONNECTED "</strong>: {v}{r}</div>";
const char HTTP_STATUS_OFFPW[] PROGMEM = "<br/>" CYD_S_PORTAL_WRONG_PASSWORD;
const char HTTP_STATUS_OFFNOAP[] PROGMEM = "<br/>" CYD_S_PORTAL_AP_NOT_FOUND;
const char HTTP_STATUS_OFFFAIL[] PROGMEM = "<br/>" CYD_S_PORTAL_NOT_CONNECTED;
const char HTTP_STATUS_NONE[] PROGMEM = "<div class='msg'>" CYD_S_PORTAL_NO_AP "</div>";
const char S_titlewifisaved[] PROGMEM = CYD_S_WIFI_SAVED;
const char S_titlewifi[] PROGMEM = CYD_S_WIFI_SETUP_TITLE;
const char S_titleexit[] PROGMEM = CYD_S_PORTAL_EXIT;
const char S_titleclose[] PROGMEM = CYD_S_WIFI_CLOSE;
const char S_nonetworks[] PROGMEM = CYD_S_WIFI_NO_NETWORKS;
const char S_exiting[] PROGMEM = CYD_S_PORTAL_EXIT;
const char S_closing[] PROGMEM = CYD_S_PORTAL_CLOSING;

#endif  // CYD_WM_STRINGS_H
