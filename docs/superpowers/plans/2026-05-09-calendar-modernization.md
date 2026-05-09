# Calendar Modernization Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the large Option C desktop Calendar modernization with richer views, drag/drop rescheduling, reminders, agent wake instructions, participants display, and client-side recurring appointment creation.

**Architecture:** Keep the existing planner API and desktop app registration. Replace only the Calendar section of the combined desktop app script, update the existing calendar CSS region, and add translation keys consumed by the new UI. Recurrence is client-side batch creation to avoid a planner schema migration.

**Tech Stack:** Vanilla JavaScript desktop SPA, embedded CSS, JSON i18n, Go static asset tests.

---

### Task 1: Regression Tests

**Files:**
- Create: `ui/desktop_calendar_modernization_test.go`

- [ ] Add a static asset test that reads `js/desktop/apps/planning-gallery-music.js` and checks for the new calendar surface markers: `vd-calendar-shell`, `data-cal-create`, `data-cal-sidebar`, `data-cal-drop-date`, `updateAppointmentDateTime`, `createRecurringAppointments`, `notification_at`, and `agent_instruction`.
- [ ] Add a CSS marker test that reads `css/desktop-apps.css` and checks for `.vd-calendar-shell`, `.vd-calendar-weekdays`, `.vd-calendar-event`, `.vd-calendar-sidebar`, `.vd-calendar-time-grid`, and the mobile calendar media query.
- [ ] Add a translation completeness test that opens every `ui/lang/desktop/*.json` file and verifies the new `desktop.cal_*` keys are present.
- [ ] Run `go test ./ui -run CalendarModernization` and confirm it fails before implementation.

### Task 2: Calendar JavaScript

**Files:**
- Create: `ui/js/desktop/apps/calendar.js`
- Modify: `ui/js/desktop/apps/planning-gallery-music.js`
- Modify: `ui/js/desktop/main.js`

- [ ] Move the calendar helpers from the old planning/gallery/music chunk into `calendar.js`.
- [ ] Load the new chunk from `ui/js/desktop/main.js`.
- [ ] Replace the current calendar host markup with a `vd-calendar-shell` layout.
- [ ] Add appointment normalization, sorting, day bucketing, status labels, and localized date range helpers.
- [ ] Modernize month, week, and day views with weekday headers, event blocks, overflow summaries, and time lanes.
- [ ] Add drag/drop rescheduling through `updateAppointmentDateTime`.
- [ ] Add status actions for complete/cancel from the modal and context menu.
- [ ] Add client-side recurring creation through `createRecurringAppointments`.
- [ ] Extend the modal to edit notification time, wake-agent instruction, recurrence controls, and existing appointment metadata.

### Task 3: Calendar CSS

**Files:**
- Modify: `ui/css/desktop-apps.css`

- [ ] Replace the current `.vd-calendar*` styling with a richer shell, command header, month grid, time grid, sidebar, event chips, modal sections, and responsive layout.
- [ ] Keep controls compact, theme-compatible, and limited to 8px or less border radius where the design permits.
- [ ] Ensure long titles truncate or wrap safely and do not resize fixed calendar grid cells.

### Task 4: Translations

**Files:**
- Modify: `ui/lang/desktop/*.json`

- [ ] Add the new calendar keys to all desktop language files.
- [ ] Use concise labels because most of these strings sit in compact desktop controls.

### Task 5: Verification And Commit

**Files:**
- Verify all modified files.

- [ ] Run `go test ./ui -run Calendar`.
- [ ] Run `go test ./ui`.
- [ ] Check `git diff --check`.
- [ ] Stage only the calendar docs, tests, JS, CSS, and desktop language files.
- [ ] Commit with a clear message.
