# Calendar Modernization Design

## Goal

Turn the desktop Calendar from a sparse appointment grid into a polished planning surface that feels worthy of the AuraGo desktop: useful at a glance, pleasant to use repeatedly, and capable of richer workflows without requiring a database migration.

## Scope

This design implements the larger "Option C" direction in the desktop app while staying on the existing appointments API. The UI will use fields that already exist on appointments: title, description, date_time, notification_at, wake_agent, agent_instruction, status, contact_ids, and participants.

Recurring events will be implemented as client-side series creation. Creating a daily, weekly, or monthly series posts multiple appointments with matching content and shifted dates. Editing one occurrence edits that appointment only. This keeps the feature shippable without changing the planner schema.

## Experience

The calendar becomes a three-part surface:

- A compact command header with previous/next, Today, New Appointment, Month/Week/Day views, and a visible date range.
- A strong calendar canvas with weekday headers, richer month cells, week/day time lanes, appointment blocks, drag/drop rescheduling, and status-aware visuals.
- A right-side planning rail with Today, Upcoming, overdue count, agent wake reminders, and participants.

The appointment modal becomes a richer editor with explicit labels, reminder date/time, wake-agent instruction, recurrence controls for new appointments, status selection, and delete/save actions that use existing themed icons.

## Data Flow

The app continues to load appointments from `GET /api/appointments?status=all`.

Creates use `POST /api/appointments`. Recurring series creation performs sequential posts so partial errors can be reported and successful earlier creations are not hidden.

Edits, drag/drop rescheduling, completion, and cancellation use `PUT /api/appointments/{id}` with only changed fields.

Deletes use `DELETE /api/appointments/{id}`.

## Files

- `ui/js/desktop/apps/calendar.js`: host the richer calendar view, modal, recurrence, and drag/drop helpers.
- `ui/js/desktop/apps/planning-gallery-music.js`: remove the old embedded calendar helpers so the legacy planning/gallery/music chunk stays small.
- `ui/js/desktop/main.js`: load the calendar chunk as part of the desktop main bundle.
- `ui/css/desktop-apps.css`: replace the current calendar CSS section with modern responsive layout styles.
- `ui/lang/desktop/*.json`: add labels for new calendar UI controls across all desktop languages.
- `ui/desktop_calendar_modernization_test.go`: add static asset regression tests for the new UX markers and translation completeness.

## Validation

Tests should prove that the desktop Calendar declares the new UI structure, keeps context/menu wiring, exposes drag/drop rescheduling, uses the richer appointment fields, and has translations for the new keys in every desktop language file.
