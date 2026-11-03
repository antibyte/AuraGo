async () => {
  let app = window.app;
  let host = document.getElementById('findit-fixture-root');
  const assert = (condition, message) => { if (!condition) throw new Error(message); };
  const wait = async (predicate, label, timeout = 10000) => {
    const until = performance.now() + timeout;
    while (performance.now() < until) {
      if (predicate()) return;
      await new Promise(resolve => setTimeout(resolve, 25));
    }
    throw new Error(`Timed out waiting for ${label}`);
  };
  const openDialog = () => host.querySelector('dialog[open]');
  const click = selector => {
    const node = host.querySelector(selector);
    assert(node && !node.disabled, `Missing or disabled control: ${selector}`);
    node.click(); return node;
  };
  const fill = (name, value, dialog = openDialog()) => {
    const field = dialog?.querySelector(`[name="${CSS.escape(name)}"]`);
    assert(field, `Missing field: ${name}`);
    field.value = String(value);
    field.dispatchEvent(new Event('input', { bubbles: true }));
    field.dispatchEvent(new Event('change', { bubbles: true }));
    return field;
  };
  const setCheck = (name, value, dialog = openDialog()) => {
    const field = dialog?.querySelector(`[name="${CSS.escape(name)}"]`);
    assert(field?.type === 'checkbox', `Missing checkbox: ${name}`);
    field.checked = !!value; field.dispatchEvent(new Event('change', { bubbles: true }));
  };
  const submit = (dialog = openDialog()) => {
    assert(dialog, 'Expected an open dialog');
    dialog.querySelector('form')?.requestSubmit();
  };
  const submitAndWait = async () => {
    const dialog = openDialog(); submit(dialog);
    await wait(() => !dialog.isConnected && !app.busy, 'dialog submission');
  };
  const command = payload => FindItModel.command({ ...payload, request_id: crypto.randomUUID() });
  const unwrap = (value, key) => value?.[key] || value;
  const findRecord = async (entity, id) => {
    const value = await FindItModel.query({ operation: 'get', entity, id });
    return value?.[entity.slice(0, -1)] || value;
  };
  const createLocation = async name => {
    const saved = await command({ operation: 'create', entity: 'locations', data: { name, kind: 'drawer', parent_id: '', description: '' } });
    return unwrap(saved, 'location');
  };
  const createItem = async (name, locationId, quantity = '') => {
    const saved = await command({ operation: 'create', entity: 'items', data: { name, category: 'electronic_component', unit: 'piece' } });
    let item = unwrap(saved, 'item');
    if (quantity) {
      await command({ operation: 'stock', entity: 'items', id: item.id, expected_version: item.version,
        data: { kind: 'receive', item_id: item.id, location_id: locationId, quantity, lot: 'audit', serial: '', condition: 'new', note: '' } });
    }
    item = await findRecord('items', item.id);
    return item;
  };
  const waitReady = () => wait(() => app.status?.enabled && !app.busy && !!host.querySelector('.fi-app'), 'FindIt ready');

  await waitReady();
  assert(app.status.can_manage, 'Audit fixture needs FindIt settings access');
  const spriteExpectations = { electronic_component: 'chip', pc_component: 'cpu', networking: 'network', server: 'storage', development: 'devboard', instruments: 'meter' };
  for (const [category, icon] of Object.entries(spriteExpectations)) {
    assert(FindItModel.iconId({ category }, app.manifest) === icon, `Category ${category} should use the ${icon} sprite`);
    assert(FindItModel.iconId({ category, template_id: 'generic' }, app.manifest) === icon, `Generic template items in ${category} should use the ${icon} sprite`);
  }

  // Read-only mode must not show import actions that silently do nothing.
  const writableStatus = app.status;
  app.status = { ...writableStatus, can_write: false, readonly: true }; app.draw();
  assert(!host.querySelector('[data-action="import-csv"]'), 'Item CSV import must be hidden in read-only mode');
  click('[data-view="projects"]'); await wait(() => app.view === 'projects' && !app.busy, 'projects view');
  assert(!host.querySelector('[data-action="import-project-csv"]'), 'BOM CSV import must be hidden in read-only mode');
  app.status = writableStatus; app.draw();

  const location = await createLocation('FindIt audit stock drawer');
  await createItem('Keyboard navigation one', location.id);
  await createItem('Keyboard navigation two', location.id);
  click('[data-view="items"]'); await wait(() => app.view === 'items' && !app.busy, 'keyboard items view');
  click('[data-action="refresh"]'); await wait(() => !app.busy && host.querySelectorAll('[data-select-item]').length >= 2, 'keyboard rows');
  const rows = [...host.querySelectorAll('.fi-list-scroll [data-select-item], .fi-item-grid [data-select-item]')];
  const key = (target, name, handled = false) => {
    const event = new KeyboardEvent('keydown', { key: name, bubbles: true, cancelable: true });
    if (handled) event.preventDefault();
    target.dispatchEvent(event);
  };
  rows[0].focus(); key(rows[0], 'ArrowDown');
  assert(document.activeElement === rows[1], 'ArrowDown must navigate inside the item list');
  rows[0].focus(); key(rows[0], 'ArrowDown', true);
  assert(document.activeElement === rows[0], 'An already handled key must not move background focus');
  const toolbar = host.querySelector('[data-action="new-item"]'); toolbar.focus(); key(toolbar, 'ArrowDown');
  assert(document.activeElement === toolbar, 'Toolbar keys must not move the item list');
  const editable = document.createElement('div'); editable.contentEditable = 'true'; editable.tabIndex = 0;
  rows[0].parentElement.append(editable); editable.focus(); key(editable, 'ArrowDown');
  assert(document.activeElement === editable, 'Editable list content must keep its keys'); editable.remove();
  click('[data-action="new-item"]'); await wait(() => !!openDialog(), 'keyboard dialog');
  const dialog = openDialog(), field = dialog.querySelector('input'); field.focus(); key(field, 'ArrowDown');
  assert(document.activeElement === field, 'Dialog keys must not navigate the background list');
  dialog.querySelector('[data-cancel]').click();
  if (dialog.isConnected) { dialog.dispatchEvent(new Event('cancel', { cancelable: true })); }
  await wait(() => !openDialog(), 'keyboard dialog closed');

  click('[data-view="items"]'); await wait(() => app.view === 'items' && !app.busy, 'items for serial validation');
  const fetchBeforeSerialValidation = window.fetch; let itemCreatesDuringValidation = 0;
  window.fetch = async (url, options = {}) => {
    let body; try { body = JSON.parse(options.body); } catch (_) { /* Other requests have no JSON body. */ }
    if (String(url).endsWith('/command') && body?.entity === 'items' && body?.operation === 'create') itemCreatesDuringValidation++;
    return fetchBeforeSerialValidation(url, options);
  };
  try {
    click('[data-action="new-item"]'); await wait(() => !!openDialog()?.querySelector('[name="track_serials"]'), 'serial-tracked item form');
    fill('name', 'FindIt audit serial validation'); fill('category', 'electronic_component'); fill('unit', 'piece');
    setCheck('track_serials', true); fill('initial_quantity', '2'); fill('initial_location_id', location.id);
    const serialForm = openDialog(); submit(serialForm);
    await wait(() => serialForm.querySelector('[data-error]') && !serialForm.querySelector('[data-error]').hidden, 'serial quantity validation');
    assert(serialForm.isConnected && itemCreatesDuringValidation === 0, 'Tracked initial stock greater than one must not create an item');
    fill('initial_quantity', '1', serialForm); submit(serialForm);
    await wait(() => !serialForm.querySelector('[data-error]').hidden, 'serial number validation');
    assert(serialForm.isConnected && itemCreatesDuringValidation === 0, 'Tracked initial stock without a serial must not create an item');
    serialForm.querySelector('[data-cancel]').click();
    await wait(() => !serialForm.isConnected, 'cancel serial validation form');
  } finally { window.fetch = fetchBeforeSerialValidation; }

  const item = await createItem('FindIt audit stock part', location.id, '8');
  const otherItem = await createItem('FindIt audit second part', location.id, '4');
  const itemAvailability = await FindItModel.query({ operation: 'availability', entity: 'items', id: item.id });

  // The confirmation must use the same item snapshot as the pending stock form.
  click('[data-view="items"]'); await wait(() => app.view === 'items' && !app.busy, 'items view');
  await app.openItem(item.id);
  click('[data-action="stock-issue"]'); await wait(() => !!openDialog()?.querySelector('[name="quantity"]'), 'issue form');
  fill('stock_id', itemAvailability.stocks[0].id); fill('quantity', '2');
  const issueForm = openDialog(); submit(issueForm);
  await wait(() => !issueForm.isConnected && !!openDialog()?.querySelector('.fi-stock-impact'), 'stock impact preview');
  const impact = openDialog().querySelector('.fi-stock-impact').textContent.replace(/\s+/g, ' ');
  assert(/8\s*→\s*6/.test(impact), `Issue preview must show 8 → 6; got: ${impact}`);
  submit(openDialog());
  await wait(() => !openDialog() && !app.busy && app.availability?.on_hand === '6', 'issue commit and refresh');
  assert((await FindItModel.query({ operation: 'availability', entity: 'items', id: item.id })).on_hand === '6', 'Issue did not reduce stock by the confirmed amount');

  // A committed stock change whose response is lost stays scoped to its exact request across window recreation.
  let lostStockResponse = false, firstStockCommand = null, retriedStockCommand = null;
  const fetchBeforeStockRecovery = window.fetch;
  window.fetch = async (url, options = {}) => {
    let body; try { body = JSON.parse(options.body); } catch (_) { /* Other requests have no JSON body. */ }
    if (String(url).endsWith('/command') && body?.operation === 'stock' && body?.entity === 'items' && body?.id === item.id && body?.data?.kind === 'issue') {
      if (!lostStockResponse) {
        lostStockResponse = true; firstStockCommand = body;
        const committed = await fetchBeforeStockRecovery(url, options);
        if (!committed.ok) return committed;
        return new Response(JSON.stringify({ code: 'unavailable', error: 'Fixture response lost after commit' }), { status: 503, headers: { 'Content-Type': 'application/json' } });
      }
      retriedStockCommand = body;
    }
    return fetchBeforeStockRecovery(url, options);
  };
  try {
    click('[data-action="stock-issue"]'); await wait(() => !!openDialog()?.querySelector('[name="quantity"]'), 'uncertain stock form');
    fill('stock_id', itemAvailability.stocks[0].id); fill('quantity', '1');
    const initialStockForm = openDialog(); submit(initialStockForm);
    await wait(() => !!openDialog()?.querySelector('.fi-stock-impact'), 'uncertain stock preview');
    submit(openDialog());
    await wait(() => lostStockResponse && !!openDialog()?.querySelector('[name="quantity"]') && !app.busy, 'exact stock retry form');
    const lockedRetry = openDialog().querySelector('[name="quantity"]');
    assert(lockedRetry.disabled, 'An uncertain stock quantity must be locked until its request is resolved');
    fill('quantity', '2'); submit(openDialog());
    await wait(() => !!openDialog()?.querySelector('.fi-stock-impact'), 'attempted stock quantity edit preview');
    submit(openDialog());
    await wait(() => !!app.error && !app.busy && app.pendingCommands.length === 1, 'changed retry blocked');
    assert(!retriedStockCommand, 'A changed retry must be blocked before sending another command');

    const originalHost = host;
    FindItApp.dispose(app.windowId);
    const reopenedHost = document.createElement('div'); reopenedHost.style.cssText = 'position:fixed;left:-2000px;top:0;width:900px;height:700px'; document.body.append(reopenedHost);
    app = FindItApp.render(reopenedHost, 'findit-audit-stock-reopened', window.findItContext); host = reopenedHost; window.app = app;
    await waitReady();
    assert(app.pendingCommands.length === 1, 'A new FindIt window did not restore the exact pending stock command');
    click('[data-pending-command]'); await wait(() => !!openDialog()?.querySelector('.fi-retry-stock-summary'), 'stock retry confirmation');
    assert(!openDialog().querySelector('[name="quantity"]'), 'The pending command confirmation must not expose editable stock fields');
    await submitAndWait();
    await wait(() => app.pendingCommands.length === 0 && !app.busy, 'pending stock retry acknowledged');
    assert(JSON.stringify(firstStockCommand) === JSON.stringify(retriedStockCommand), 'Stock recovery changed the saved command payload or request ID');
    const recoveredStock = await FindItModel.query({ operation: 'availability', entity: 'items', id: item.id });
    assert(recoveredStock.on_hand === '5' && recoveredStock.stocks.length === 1, 'Stock recovery duplicated or lost a committed issue');
    FindItApp.dispose(app.windowId); reopenedHost.remove();
    host = originalHost; app = FindItApp.render(originalHost, 'test', window.findItContext); window.app = app;
    await waitReady();
  } finally { window.fetch = fetchBeforeStockRecovery; }

  // A shell deep link can change the selection while a stock form is open. Never apply its stale form.
  const beforeStaleStock = (await FindItModel.query({ operation: 'availability', entity: 'items', id: item.id })).on_hand;
  await app.openItem(item.id);
  click('[data-action="stock-issue"]'); await wait(() => !!openDialog()?.querySelector('[name="quantity"]'), 'stale issue form');
  fill('quantity', '1');
  await FindItApp.open(app.windowId, { item_id: otherItem.id });
  await wait(() => app.selected?.id === otherItem.id && !app.busy, 'deep link selection change');
  const staleForm = openDialog(); submit(staleForm);
  await wait(() => !staleForm.isConnected && !openDialog() && !app.busy, 'stale stock form cancellation');
  assert((await FindItModel.query({ operation: 'availability', entity: 'items', id: item.id })).on_hand === beforeStaleStock, 'Stale stock form changed the old item');

  const projectCreate = await command({ operation: 'create', entity: 'projects', data: { name: 'FindIt audit reserve target', description: 'Selection race' } });
  let project = unwrap(projectCreate, 'project');
  const projectUpdate = await command({ operation: 'update', entity: 'projects', id: project.id, expected_version: project.version,
    data: { name: project.name, description: project.description, lines: [{ id: '', item_id: item.id, quantity: '1', note: '' }] } });
  project = unwrap(projectUpdate, 'project');
  const released = await command({ operation: 'project_status', entity: 'projects', id: project.id, expected_version: project.version, data: { status: 'released' } });
  project = unwrap(released, 'project');
  const otherProjectResult = await command({ operation: 'create', entity: 'projects', data: { name: 'FindIt audit other project', description: 'Selection race' } });
  const otherProject = unwrap(otherProjectResult, 'project');

  // Changing projects while availability is loading must cancel the stale reservation dialog.
  click('[data-view="projects"]'); await wait(() => app.view === 'projects' && !app.busy, 'projects for reservation race');
  click(`[data-select-project="${CSS.escape(project.id)}"]`); await wait(() => app.project?.id === project.id && !app.busy, 'reserve target selected');
  const line = app.project.lines[0];
  const fetchBefore = window.fetch; let availabilitySeen = false, availabilityFinished = false, releaseAvailability;
  window.fetch = async (url, options = {}) => {
    let body; try { body = JSON.parse(options.body); } catch (_) { /* Other requests have no JSON body. */ }
    if (!availabilitySeen && String(url).endsWith('/query') && body?.operation === 'availability' && body.id === item.id) {
      availabilitySeen = true;
      await new Promise(resolve => { releaseAvailability = resolve; });
      const response = await fetchBefore(url, options); availabilityFinished = true; return response;
    }
    return fetchBefore(url, options);
  };
  try {
    click(`[data-reserve-line="${CSS.escape(line.id)}"]`);
    await wait(() => availabilitySeen, 'delayed reservation availability');
    click(`[data-select-project="${CSS.escape(otherProject.id)}"]`);
    await wait(() => app.project?.id === otherProject.id && !app.busy, 'second project selection');
    releaseAvailability();
    await wait(() => availabilityFinished, 'delayed availability response');
    await wait(() => !openDialog() && !app.busy, 'stale reservation cancellation');
    assert(app.project?.id === otherProject.id, 'Stale reservation flow changed the selected project');
    const unchanged = await FindItModel.query({ operation: 'availability', entity: 'items', id: item.id });
    assert(unchanged.reserved === '0', 'Stale reservation flow created a hold on the earlier project');
  } finally { releaseAvailability?.(); window.fetch = fetchBefore; }
  click(`[data-select-project="${CSS.escape(project.id)}"]`); await wait(() => app.project?.id === project.id && !app.busy, 'reserve target restored');
  const fetchBeforeFailure = window.fetch;
  window.fetch = async (url, options = {}) => {
    let body; try { body = JSON.parse(options.body); } catch (_) { /* Other requests have no JSON body. */ }
    if (String(url).endsWith('/query') && body?.operation === 'availability' && body.id === item.id) {
      return new Response(JSON.stringify({ code: 'unavailable', error: 'Fixture availability failure' }), { status: 503, headers: { 'Content-Type': 'application/json' } });
    }
    return fetchBeforeFailure(url, options);
  };
  try {
    click(`[data-reserve-line="${CSS.escape(line.id)}"]`);
    await wait(() => !!app.error && !app.busy, 'reservation availability error feedback');
    assert(!openDialog(), 'Reservation availability failure opened an unusable form');
  } finally { window.fetch = fetchBeforeFailure; }

  // A reservation committed before a lost response must keep its exact request ID and payload.
  let lostReservationResponse = false, firstReservationCommand = null, retriedReservationCommand = null;
  const fetchBeforeReservationRecovery = window.fetch;
  window.fetch = async (url, options = {}) => {
    let body; try { body = JSON.parse(options.body); } catch (_) { /* Other requests have no JSON body. */ }
    if (String(url).endsWith('/command') && body?.operation === 'reserve' && body?.entity === 'projects' && body?.id === project.id) {
      if (!lostReservationResponse) {
        lostReservationResponse = true; firstReservationCommand = body;
        const committed = await fetchBeforeReservationRecovery(url, options);
        if (!committed.ok) return committed;
        return new Response(JSON.stringify({ code: 'unavailable', error: 'Fixture reservation response lost after commit' }), { status: 503, headers: { 'Content-Type': 'application/json' } });
      }
      retriedReservationCommand = body;
    }
    return fetchBeforeReservationRecovery(url, options);
  };
  try {
    click(`[data-reserve-line="${CSS.escape(line.id)}"]`); await wait(() => !!openDialog()?.querySelector('[name="quantity"]'), 'reservation form');
    fill('quantity', '1'); const reservationForm = openDialog(); submit(reservationForm);
    await wait(() => lostReservationResponse && !!openDialog()?.querySelector('[name="quantity"]') && !app.busy, 'exact reservation retry form');
    assert(openDialog().querySelector('[name="quantity"]').disabled, 'An uncertain reservation quantity must be locked');
    openDialog().querySelector('[data-cancel]').click(); await wait(() => !openDialog(), 'close exact reservation form');
    click('[data-pending-command]'); await wait(() => !!openDialog()?.querySelector('.fi-retry-stock-summary'), 'saved reservation confirmation');
    await submitAndWait(); await wait(() => app.pendingCommands.length === 0 && !app.busy, 'reservation retry acknowledged');
    assert(JSON.stringify(firstReservationCommand) === JSON.stringify(retriedReservationCommand), 'Reservation recovery changed the saved payload or request ID');
    const reservedAvailability = await FindItModel.query({ operation: 'availability', entity: 'items', id: item.id });
    assert(reservedAvailability.reserved === '1', 'Reservation recovery duplicated or lost the committed reservation');
  } finally { window.fetch = fetchBeforeReservationRecovery; }

  // A stale retry confirmation in another FindIt window must not resurrect a request already rejected elsewhere.
  const staleReserveCreated = await command({ operation: 'create', entity: 'projects', data: { name: 'FindIt stale retry audit', description: 'Cross-window rejection' } });
  let staleReserveProject = unwrap(staleReserveCreated, 'project');
  const staleReserveUpdated = await command({ operation: 'update', entity: 'projects', id: staleReserveProject.id, expected_version: staleReserveProject.version,
    data: { name: staleReserveProject.name, description: staleReserveProject.description, lines: [{ id: '', item_id: item.id, quantity: '1', note: '' }] } });
  staleReserveProject = unwrap(staleReserveUpdated, 'project');
  const staleReserveReleased = await command({ operation: 'project_status', entity: 'projects', id: staleReserveProject.id, expected_version: staleReserveProject.version, data: { status: 'released' } });
  staleReserveProject = unwrap(staleReserveReleased, 'project');
  click('[data-action="refresh"]'); await wait(() => !app.busy, 'refresh projects for stale retry test');
  click(`[data-select-project="${CSS.escape(staleReserveProject.id)}"]`); await wait(() => app.project?.id === staleReserveProject.id && !app.busy, 'stale retry project selected');
  const staleReserveLine = app.project.lines[0];
  let firstStaleReserve = null; const staleReserveCalls = [];
  const fetchBeforeStaleRetry = window.fetch;
  window.fetch = async (url, options = {}) => {
    let body; try { body = JSON.parse(options.body); } catch (_) { /* Other requests have no JSON body. */ }
    if (String(url).endsWith('/command') && body?.operation === 'reserve' && body?.id === staleReserveProject.id) {
      staleReserveCalls.push(body);
      if (!firstStaleReserve) {
        firstStaleReserve = body;
        return new Response(JSON.stringify({ code: 'unavailable', error: 'Fixture reservation was not committed' }), { status: 503, headers: { 'Content-Type': 'application/json' } });
      }
    }
    return fetchBeforeStaleRetry(url, options);
  };
  try {
    click(`[data-reserve-line="${CSS.escape(staleReserveLine.id)}"]`); await wait(() => !!openDialog()?.querySelector('[name="quantity"]'), 'unresolved reservation form');
    fill('quantity', '1'); submit(openDialog());
    await wait(() => staleReserveCalls.length === 1 && !!openDialog()?.querySelector('[name="quantity"]') && !app.busy, 'reservation exact retry after not committed response');
    openDialog().querySelector('[data-cancel]').click(); await wait(() => !openDialog(), 'close reservation exact retry form');
    click('[data-pending-command]'); await wait(() => !!openDialog()?.querySelector('.fi-retry-stock-summary'), 'first window retry confirmation');
    const staleConfirmation = openDialog();
    const firstHost = host, firstApp = app;
    firstApp.error = ''; firstApp.busy = true;
    staleConfirmation.querySelector('[data-submit]').click();
    await new Promise(resolve => setTimeout(resolve, 40));
    assert(firstApp.busy && !firstApp.actionBusy, 'First-window exact retry should be waiting behind the simulated busy operation');

    const freshProject = await findRecord('projects', staleReserveProject.id);
    await command({ operation: 'project_status', entity: 'projects', id: freshProject.id, expected_version: freshProject.version, data: { status: 'cancelled' } });
    const secondHost = document.createElement('div'); secondHost.style.cssText = 'position:fixed;left:-2000px;top:0;width:900px;height:700px'; document.body.append(secondHost);
    const secondApp = FindItApp.render(secondHost, 'findit-stale-retry-window', window.findItContext);
    await wait(() => secondApp.status?.enabled && !secondApp.busy && !!secondHost.querySelector('.fi-app'), 'second stale retry window ready');
    assert(secondApp.pendingCommands.length === 1, 'Second window did not restore the pending reservation');
    secondHost.querySelector('[data-pending-command]')?.click(); await wait(() => !!secondHost.querySelector('dialog[open]'), 'second window retry confirmation');
    const secondConfirmation = secondHost.querySelector('dialog[open]'); secondConfirmation.querySelector('[data-submit]').click();
    await wait(() => secondApp.pendingCommands.length === 0 && !!secondApp.error && !secondApp.busy && !secondApp.actionBusy, 'reservation definitively rejected in second window');
    assert(staleReserveCalls.length === 2 && staleReserveCalls[1].request_id === staleReserveCalls[0].request_id, 'Second window did not resolve the same reservation receipt');
    firstApp.busy = false; firstApp.draw();
    await wait(() => !!firstApp.error && !firstApp.busy && !firstApp.actionBusy, 'stale first-window replay blocked after busy wait');
    assert(staleReserveCalls.length === 2, 'Stale confirmation resent a definitively rejected reservation');
    FindItApp.dispose(secondApp.windowId); secondHost.remove();
    app = firstApp; host = firstHost; window.app = app;
  } finally { window.fetch = fetchBeforeStaleRetry; }

  // Changing the selected record while the BOM item picker is open must cancel the stale add-line flow.
  const pickerProjectCreated = await command({ operation: 'create', entity: 'projects', data: { name: 'FindIt picker selection audit', description: 'Stale picker' } });
  const pickerProject = unwrap(pickerProjectCreated, 'project');
  click('[data-action="refresh"]'); await wait(() => !app.busy, 'refresh projects for picker selection test');
  click(`[data-select-project="${CSS.escape(pickerProject.id)}"]`); await wait(() => app.project?.id === pickerProject.id && !app.busy, 'picker project selected');
  const addLinePromise = app.features.projects.addProjectLine();
  await wait(() => !!openDialog()?.querySelector('[data-picker-search]'), 'BOM item picker');
  await FindItApp.open(app.windowId, { item_id: item.id }); await wait(() => app.selected?.id === item.id && !app.busy, 'selection changed during picker');
  await wait(() => !!openDialog()?.querySelector('[data-pick-item]'), 'picker item results');
  openDialog().querySelector('[data-pick-item]').click(); await addLinePromise;
  assert(!openDialog(), 'Stale BOM picker should not open the quantity form after selection changes');
  assert((await findRecord('projects', pickerProject.id))?.lines?.length === 0, 'Stale BOM picker changed the previous project');

  // Filtering from a later page must return to page one.
  for (let index = 0; index < 31; index++) await command({ operation: 'create', entity: 'items', data: {
    name: `FindIt pagination audit ${String(index + 1).padStart(2, '0')}`, category: 'electronic_component', unit: 'piece'
  } });
  click('[data-view="items"]'); await wait(() => app.view === 'items' && !app.busy, 'items for pagination');
  if (!app.filtersOpen) click('[data-action="toggle-filters"]');
  click('[data-action="clear-filters"]'); await wait(() => app.offset === 0 && !app.busy, 'filter reset');
  await wait(() => app.total > app.limit, 'more than one item page');
  click('[data-page="next"]'); await wait(() => app.offset === app.limit && !app.busy, 'second item page');
  const category = host.querySelector('[data-filter-category]');
  category.value = 'electronic_component'; category.dispatchEvent(new Event('change', { bubbles: true }));
  await wait(() => app.offset === 0 && !app.busy, 'filter resets pagination');

  // The location filter must stay reachable in the filter panel, because the sidebar tree is hidden on narrow windows.
  const locationFilter = host.querySelector('[data-filter-location]');
  assert(locationFilter, 'Location filter must be present in the filter panel');
  const locationOptions = [...locationFilter.options].map(option => option.value).filter(Boolean);
  assert(locationOptions.length > 0, 'Location filter must offer the stored locations');
  const locationWithStock = app.locations.find(location => locationOptions.includes(location.id));
  if (locationWithStock) {
    locationFilter.value = locationWithStock.id; locationFilter.dispatchEvent(new Event('change', { bubbles: true }));
    await wait(() => app.locationId === locationWithStock.id && app.offset === 0 && !app.busy, 'location filter applies');
    click('[data-action="clear-filters"]'); await wait(() => app.locationId === '' && !app.busy, 'clear filters resets the location filter');
  }

  // List columns are addressed by key, so a narrow layout keeps hiding the same columns for a custom column set.
  const headers = [...host.querySelectorAll('.fi-item-table thead th[data-column]')];
  assert(headers.length > 0 && headers.every(header => header.dataset.column), 'Item table columns must carry their key');
  const partHeader = host.querySelector('.fi-item-table thead th[data-column="part"]');
  assert(partHeader, 'The item table must keep a part column');

  // A committed opening-stock response loss must remain retryable in a newly opened window.
  const originalHost = document.getElementById('findit-fixture-root');
  const createName = 'FindIt opening-stock recovery audit';
  let itemId = '', lostResponseBody = null, retryBody = null, lost = false;
  const fetchBeforeRecovery = window.fetch;
  window.fetch = async (url, options = {}) => {
    let body; try { body = JSON.parse(options.body); } catch (_) { /* Not a JSON request. */ }
    if (String(url).endsWith('/command') && body?.entity === 'items' && body?.operation === 'create') {
      const response = await fetchBeforeRecovery(url, options);
      if (response.ok) { const saved = await response.clone().json(); itemId = (saved.item || saved).id; }
      return response;
    }
    if (String(url).endsWith('/command') && body?.entity === 'items' && body?.operation === 'stock' && body.data?.item_id === itemId) {
      if (!lost) {
        lost = true; lostResponseBody = body;
        const committed = await fetchBeforeRecovery(url, options);
        if (!committed.ok) return committed;
        return new Response(JSON.stringify({ code: 'unavailable', error: 'Fixture response lost after commit' }), { status: 503, headers: { 'Content-Type': 'application/json' } });
      }
      retryBody = body;
    }
    return fetchBeforeRecovery(url, options);
  };
  try {
    click('[data-action="new-item"]'); await wait(() => !!openDialog(), 'opening-stock item form');
    fill('name', createName); fill('category', 'electronic_component'); fill('unit', 'piece');
    fill('initial_quantity', '7'); fill('initial_location_id', location.id); await submitAndWait();
    await wait(() => lost && app.pendingInitialStock?.item_id === itemId, 'opening-stock receipt retained');
    const receiptId = app.pendingInitialStock.request_id;
    FindItApp.dispose(app.windowId);
    const reopenedHost = document.createElement('div'); reopenedHost.style.cssText = 'position:fixed;left:-2000px;top:0;width:900px;height:700px'; document.body.append(reopenedHost);
    app = FindItApp.render(reopenedHost, 'findit-audit-reopened', window.findItContext); host = reopenedHost; window.app = app;
    await waitReady();
    assert(app.pendingInitialStock?.item_id === itemId && app.pendingInitialStock.request_id === receiptId, 'New window did not restore the exact opening-stock receipt');
    const reopenedWritableStatus = app.status;
    app.status = { ...reopenedWritableStatus, can_write: false, readonly: true }; app.draw();
    assert(host.querySelector('[data-action="retry-initial-stock"]')?.disabled, 'Opening-stock retry must be disabled without write authorization');
    app.status = reopenedWritableStatus; app.draw();
    click('[data-action="retry-initial-stock"]'); await wait(() => !!openDialog()?.querySelector('.fi-retry-stock-summary'), 'opening-stock exact-retry confirmation');
    assert(!openDialog().querySelector('[name="quantity"]'), 'An uncertain opening-stock receipt must not be editable before replay');
    assert(openDialog().textContent.includes('7'), 'Retry confirmation must show the saved opening quantity');
    await submitAndWait();
    await wait(() => !app.pendingInitialStocks.length && !app.busy, 'opening-stock retry acknowledged');
    assert(lostResponseBody?.request_id === retryBody?.request_id && lostResponseBody?.request_id === receiptId, 'Reopened retry changed its request ID');
    assert(JSON.stringify(lostResponseBody) === JSON.stringify(retryBody), 'Reopened retry changed the committed command payload');
    const recovered = await FindItModel.query({ operation: 'availability', entity: 'items', id: itemId });
    assert(recovered.on_hand === '7' && recovered.stocks.length === 1, 'Opening-stock recovery duplicated or lost stock');

    // A definitive rejection clears only the exact failed receipt and leaves the item available for normal stock entry.
    const archivedLocation = await createLocation('FindIt audit archived stock location');
    await command({ operation: 'archive', entity: 'locations', id: archivedLocation.id, expected_version: archivedLocation.version, data: {} });
    const rejectedItem = await createItem('FindIt audit rejected opening stock', '');
    const rejectedReceipt = {
      item_id: rejectedItem.id, item_name: rejectedItem.name, item_version: rejectedItem.version, unit: rejectedItem.unit,
      track_serials: false, request_id: crypto.randomUUID(),
      stock_data: { kind: 'receive', item_id: rejectedItem.id, location_id: archivedLocation.id, quantity: '1', lot: '', serial: '', condition: 'new', note: '' }
    };
    assert(app.persistPendingInitialStock(rejectedReceipt), 'Could not seed the pending opening-stock receipt');
    let rejectedResponse = null;
    const fetchBeforeRejection = window.fetch;
    window.fetch = async (url, options = {}) => {
      let body; try { body = JSON.parse(options.body); } catch (_) { /* Other requests have no JSON body. */ }
      const response = await fetchBeforeRejection(url, options);
      if (String(url).endsWith('/command') && body?.request_id === rejectedReceipt.request_id) {
        rejectedResponse = { status: response.status, body: await response.clone().json() };
      }
      return response;
    };
    try {
      click('[data-action="retry-initial-stock"]'); await wait(() => !!openDialog()?.querySelector('.fi-retry-stock-summary'), 'rejected receipt confirmation');
      await submitAndWait();
      await wait(() => !app.pendingInitialStocks.length && !!app.error && !app.busy && !app.actionBusy, 'definitive opening-stock rejection handled');
      assert(rejectedResponse?.status === 404 && rejectedResponse.body?.code === 'not_found', `Expected definitive archived-location rejection, got ${JSON.stringify(rejectedResponse)}`);
      assert(!host.querySelector('[data-action="retry-initial-stock"]'), 'A definitively rejected receipt must be removed from recovery');
      const capture = host.querySelector('[data-action="capture"]');
      assert(capture && !capture.disabled, `A definitive rejection must unblock normal item entry (status=${JSON.stringify({ can_write: app.status?.can_write, readonly: app.status?.readonly, context_readonly: app.ctx?.readonly, pending: app.pendingInitialStocks.length })})`);
      assert((await findRecord('items', rejectedItem.id))?.id === rejectedItem.id, 'Rejected opening stock must leave its item saved');
    } finally { window.fetch = fetchBeforeRejection; }
    FindItApp.dispose(app.windowId); reopenedHost.remove();
    host = originalHost; app = FindItApp.render(originalHost, 'test', window.findItContext); window.app = app;
    await waitReady();
  } finally { window.fetch = fetchBeforeRecovery; }

  assert(window.errors.length === 0, `browser errors: ${window.errors.join('\n')}`);
  return { readonlyImports: 'hidden', stockIssue: '8 → 6 confirmed and committed', staleStock: 'cancelled', staleReservation: 'cancelled',
    pagination: 'filter returned to page one', openingStockRecovery: 'same receipt retried once after reopen' };
}
