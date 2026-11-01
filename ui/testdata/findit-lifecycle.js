async () => {
  const app = window.app, host = document.getElementById('findit-fixture-root');
  if (typeof window.host !== 'undefined') throw new Error('Lifecycle fixture must not expose a global host binding');
  const assert = (value, message) => { if (!value) throw new Error(message); };
  const click = selector => {
    const node = host.querySelector(selector);
    assert(node && !node.disabled, 'Missing or disabled control: ' + selector);
    node.click(); return node;
  };
  const fill = (name, value, dialog = host.querySelector('dialog[open]')) => {
    const field = dialog?.querySelector(`[name="${CSS.escape(name)}"]`);
    assert(field, 'Missing lifecycle form field: ' + name);
    field.value = String(value); field.dispatchEvent(new Event('input', { bubbles: true })); field.dispatchEvent(new Event('change', { bubbles: true }));
    return field;
  };
  const submit = () => {
    const dialog = host.querySelector('dialog[open]');
    assert(dialog, 'Expected an open lifecycle form');
    dialog.querySelector('form').requestSubmit();
  };
  const openDialog = () => host.querySelector('dialog[open]');
  function interceptCommittedWrite(entity, operation) {
    const requests = []; let failedAfterCommit = false, first = true;
    window.fetch = async (url, options = {}) => {
      let body; try { body = JSON.parse(options.body); } catch (_) { /* Not a JSON request. */ }
      if (String(url).endsWith('/command') && body?.entity === entity && body?.operation === operation) {
        requests.push(body);
        if (first) {
          first = false;
          const committed = await fetchBefore(url, options);
          if (!committed.ok) return committed;
          failedAfterCommit = true;
          return new Response(JSON.stringify({ code: 'unavailable', error: 'Fixture response lost after commit' }), { status: 503, headers: { 'Content-Type': 'application/json' } });
        }
      }
      return fetchBefore(url, options);
    };
    return { requests, failed: () => failedAfterCommit, exactRetry: () => requests.length === 2 && JSON.stringify(requests[0]) === JSON.stringify(requests[1]) };
  }
  const wait = async (predicate, label) => {
    const end = performance.now() + 10000;
    while (!predicate()) {
      if (performance.now() > end) throw new Error('Timeout: ' + label);
      await new Promise(resolve => setTimeout(resolve, 25));
    }
  };
  const items = (await FindItModel.query({ operation: 'search', entity: 'items' })).items;
  const first = items.find(item => item.part_number === 'LM358P'), second = items.find(item => item.name === 'Bench instrument');
  assert(first && second, 'Lifecycle fixture needs two saved records');

  // A failed earlier selection must never clear the later successful selection.
  const fetchBefore = window.fetch;
  let release, intercepted = false;
  window.fetch = async (url, options = {}) => {
    let body; try { body = JSON.parse(options.body); } catch (_) { /* Not a JSON request. */ }
    if (String(url).endsWith('/query') && body?.operation === 'get' && body.id === first.id) {
      intercepted = true;
      await new Promise(resolve => { release = resolve; });
      return new Response(JSON.stringify({ code: 'unavailable', error: 'Fixture unavailable' }), { status: 503, headers: { 'Content-Type': 'application/json' } });
    }
    return fetchBefore(url, options);
  };
  try {
    const older = app.openItem(first.id);
    await wait(() => intercepted, 'delayed first selection');
    await app.openItem(second.id);
    release(); await older;
    assert(app.selected?.id === second.id && !app.error, 'A late error replaced the current selection');
  } finally { release?.(); window.fetch = fetchBefore; }

  // Rendering and a narrow host retain the same open form and its draft.
  host.querySelector('[data-action="new-item"]').click();
  await wait(() => !!host.querySelector('dialog[open]'), 'draft dialog');
  const draft = host.querySelector('dialog[open]'), name = draft.querySelector('[name="name"]');
  name.value = 'Unsaved calibration adapter'; name.dispatchEvent(new Event('input', { bubbles: true }));
  await app.openItem(first.id);
  host.style.width = '360px';
  await new Promise(resolve => requestAnimationFrame(() => requestAnimationFrame(resolve)));
  assert(draft.isConnected && draft.open && name.value === 'Unsaved calibration adapter', 'Redraw or resize lost the open draft');
  draft.querySelector('[data-cancel]').click(); host.style.width = '';

  // A failed write returns to the complete draft and an explicit retry creates once.
  let rejectNextCreate = true;
  window.fetch = async (url, options = {}) => {
    let body; try { body = JSON.parse(options.body); } catch (_) { /* Not JSON. */ }
    if (rejectNextCreate && String(url).endsWith('/command') && body?.operation === 'create' && body.entity === 'items') {
      rejectNextCreate = false;
      return new Response(JSON.stringify({ code: 'unavailable', error: 'Fixture connection failure' }), { status: 503, headers: { 'Content-Type': 'application/json' } });
    }
    return fetchBefore(url, options);
  };
  try {
    host.querySelector('[data-action="new-item"]').click();
    await wait(() => !!host.querySelector('dialog[open]'), 'retry draft');
    let form = host.querySelector('dialog[open] form');
    form.elements.name.value = 'Retry-safe accessory'; form.elements.name.dispatchEvent(new Event('input', { bubbles: true }));
    form.requestSubmit();
    await wait(() => !rejectNextCreate && !app.busy && host.querySelector('dialog[open] [name="name"]')?.value === 'Retry-safe accessory', 'failed-write draft restored');
    form = host.querySelector('dialog[open] form'); form.requestSubmit();
    await wait(() => !host.querySelector('dialog[open]') && !app.busy && app.selected?.name === 'Retry-safe accessory', 'explicit retry saved');
    const matches = await FindItModel.query({ operation: 'search', entity: 'items', query: 'Retry-safe accessory' });
    assert(matches.total === 1, 'Explicit retry duplicated the item');
    await app.openItem(first.id);
  } finally { window.fetch = fetchBefore; }

  // Project and location create forms retain their drafts and replay the exact command after a lost response.
  try {
    const projectName = 'Retry-safe lifecycle project';
    click('[data-view="projects"]'); await wait(() => app.view === 'projects' && !app.busy, 'projects for retry');
    click('[data-action="new-project"]'); await wait(() => !!openDialog(), 'project create form');
    fill('name', projectName); fill('description', 'Keep this draft after an uncertain save');
    const projectWrite = interceptCommittedWrite('projects', 'create');
    submit();
    await wait(() => projectWrite.failed() && openDialog()?.querySelector('[name="name"]')?.value === projectName, 'project create draft restored');
    assert(openDialog().querySelector('[name="description"]').value === 'Keep this draft after an uncertain save', 'Project description draft was lost');
    submit();
    await wait(() => !openDialog() && app.project?.name === projectName && !app.busy, 'project exact retry');
    assert(projectWrite.exactRetry(), 'Project retry must replay the identical payload and request ID');
    const projects = await FindItModel.query({ operation: 'search', entity: 'projects', query: projectName });
    assert(projects.total === 1, 'Project retry after a committed response loss created a duplicate');
    const projectId = app.project.id;
    window.fetch = fetchBefore;

    // A successful write followed by a failed list refresh must not reopen the mutation form.
    const refreshFailureName = 'Acknowledged project with failed refresh';
    let createCount = 0, failNextProjectSearch = true;
    window.fetch = async (url, options = {}) => {
      let body; try { body = JSON.parse(options.body); } catch (_) { /* Not JSON. */ }
      if (String(url).endsWith('/command') && body?.entity === 'projects' && body?.operation === 'create') createCount++;
      if (failNextProjectSearch && String(url).endsWith('/query') && body?.entity === 'projects' && body?.operation === 'search') {
        failNextProjectSearch = false;
        return new Response(JSON.stringify({ code: 'unavailable', error: 'Fixture refresh failure' }), { status: 503, headers: { 'Content-Type': 'application/json' } });
      }
      return fetchBefore(url, options);
    };
    click('[data-action="new-project"]'); await wait(() => !!openDialog(), 'project create after refresh failure');
    fill('name', refreshFailureName); submit();
    await wait(() => createCount === 1 && !openDialog() && !app.busy, 'acknowledged project stays closed after refresh failure');
    const refreshedFailureProjects = await FindItModel.query({ operation: 'search', entity: 'projects', query: refreshFailureName });
    assert(refreshedFailureProjects.total === 1 && createCount === 1, 'A refresh error retried an already acknowledged project create');
    window.fetch = fetchBefore;

    click('[data-view="locations"]'); await wait(() => app.view === 'locations' && !app.busy, 'locations for retry');
    click('[data-action="new-location"]'); await wait(() => !!openDialog(), 'location create form');
    fill('name', 'Retry-safe lifecycle drawer'); fill('kind', 'drawer'); fill('description', 'Keep the bin description');
    const locationWrite = interceptCommittedWrite('locations', 'create');
    submit();
    await wait(() => locationWrite.failed() && openDialog()?.querySelector('[name="name"]')?.value === 'Retry-safe lifecycle drawer', 'location create draft restored');
    assert(openDialog().querySelector('[name="description"]').value === 'Keep the bin description' && openDialog().querySelector('[name="kind"]').value === 'drawer', 'Location draft fields were lost');
    submit();
    await wait(() => !openDialog() && app.locations.some(row => row.name === 'Retry-safe lifecycle drawer') && !app.busy, 'location exact retry');
    assert(locationWrite.exactRetry(), 'Location retry must replay the identical payload and request ID');
    const locations = await FindItModel.query({ operation: 'search', entity: 'locations', query: 'Retry-safe lifecycle drawer' });
    assert(locations.total === 1, 'Location retry after a committed response loss created a duplicate');
    window.fetch = fetchBefore;

    // BOM addition keeps the selected item, quantity and note for an exact retry.
    click('[data-view="projects"]'); await wait(() => app.view === 'projects' && !app.busy, 'projects after location retry');
    click(`[data-select-project="${CSS.escape(projectId)}"]`); await wait(() => app.project?.id === projectId && !app.busy, 'retry project selection');
    click('[data-action="add-project-line"]');
    await wait(() => !!host.querySelector('.fi-picker-dialog[open] [data-pick-item]'), 'BOM item picker');
    click(`.fi-picker-dialog[open] [data-pick-item="${CSS.escape(first.id)}"]`);
    await wait(() => !!openDialog()?.querySelector('[name="quantity"]'), 'BOM line form');
    fill('quantity', '3'); fill('note', 'Retained selected part and note');
    const lineWrite = interceptCommittedWrite('projects', 'update');
    submit();
    await wait(() => lineWrite.failed() && openDialog()?.querySelector('[name="quantity"]')?.value === '3', 'BOM line draft restored');
    assert(openDialog().querySelector('[name="note"]').value === 'Retained selected part and note', 'BOM note draft was lost');
    submit();
    await wait(() => !openDialog() && app.project?.lines?.some(line => line.item_id === first.id && line.quantity === '3') && !app.busy, 'BOM line exact retry');
    assert(lineWrite.exactRetry(), 'BOM line retry must replay the identical payload and request ID');
    window.fetch = fetchBefore;

    const line = app.project.lines.find(value => value.item_id === first.id);
    click('[data-action="release-project"]'); await wait(() => !!openDialog(), 'release lifecycle project'); submit();
    await wait(() => app.project?.status === 'released' && !app.busy, 'release lifecycle project');

    // Reservation and release dialogs retain their selected stock and quantities too.
    click(`[data-reserve-line="${CSS.escape(line.id)}"]`);
    await wait(() => !!openDialog()?.querySelector('[name="stock_id"]'), 'reservation form');
    const stockID = openDialog().querySelector('[name="stock_id"]').value;
    fill('quantity', '2');
    const reservationWrite = interceptCommittedWrite('projects', 'reserve');
    submit();
    await wait(() => reservationWrite.failed() && openDialog()?.querySelector('[name="stock_id"]')?.value === stockID && openDialog()?.querySelector('[name="quantity"]')?.value === '2', 'reservation draft restored');
    submit();
    await wait(() => !openDialog() && app.project?.reservations?.some(item => item.line_id === line.id && item.remaining === '2') && !app.busy, 'reservation exact retry');
    assert(reservationWrite.exactRetry(), 'Reservation retry must replay the identical stock choice, quantity and request ID');
    window.fetch = fetchBefore;

    const reservation = app.project.reservations.find(item => item.line_id === line.id && item.remaining === '2');
    click(`[data-reservation-action="release"][data-reservation-id="${CSS.escape(reservation.id)}"]`);
    await wait(() => !!openDialog()?.querySelector('[name="quantity"]'), 'partial release form');
    fill('quantity', '1');
    const releaseWrite = interceptCommittedWrite('projects', 'release');
    submit();
    await wait(() => releaseWrite.failed() && openDialog()?.querySelector('[name="quantity"]')?.value === '1', 'partial release draft restored');
    submit();
    await wait(() => !openDialog() && app.project?.reservations?.some(item => item.id === reservation.id && item.remaining === '1') && !app.busy, 'partial release exact retry');
    assert(releaseWrite.exactRetry(), 'Reservation release retry must replay the identical payload and request ID');
  } finally { window.fetch = fetchBefore; }

  // Two mounted windows have isolated state, dialog IDs, and disposal boundaries.
  click('button[data-view="items"]');
  await wait(() => app.view === 'items' && !app.busy, 'return to workbench after project retries');
  await app.openItem(first.id);
  const otherHost = document.createElement('div'); otherHost.id = 'other-host';
  otherHost.style.cssText = 'position:fixed;left:-2000px;top:0;width:400px;height:800px'; document.body.append(otherHost);
  const other = FindItApp.render(otherHost, 'second-window', window.findItContext);
  try {
    await wait(() => other.status && !other.busy, 'second window ready');
    await other.openItem(second.id);
    assert(app.selected?.id === first.id && other.selected?.id === second.id, 'Windows share the selected item');
    const one = FindItDialogs.openForm({ host, title: 'First', fields: [{ key: 'name', label: 'Name' }], signal: app.signal });
    const two = FindItDialogs.openForm({ host: otherHost, title: 'Second', fields: [{ key: 'name', label: 'Name' }], signal: other.signal });
    const firstDialog = host.querySelector('dialog[open]'), secondDialog = otherHost.querySelector('dialog[open]');
    assert(firstDialog.getAttribute('aria-labelledby') !== secondDialog.getAttribute('aria-labelledby'), 'Dialog heading IDs collide across windows');
    assert(firstDialog.querySelector('input').id !== secondDialog.querySelector('input').id, 'Form field IDs collide across windows');
    FindItApp.dispose('second-window'); await two;
    assert(!secondDialog.isConnected && firstDialog.isConnected && !app.signal.aborted, 'Disposing a window affected another window');
    firstDialog.querySelector('[data-cancel]').click(); await one;
  } finally { FindItApp.dispose('second-window'); otherHost.remove(); }
  assert(!host.querySelector('dialog[open]'), 'A disposed workflow leaked a dialog');
  return { staleResponse: 'ignored', draft: 'retained through redraw/resize and uncertain project/location/BOM/reservation retries', windows: 'isolated state and teardown' };
}
