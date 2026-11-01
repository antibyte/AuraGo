async () => {
  const app = window.app;
  if (!app || !window.FindItApp) throw new Error('FindIt fixture did not mount the app');
  const wait = async (predicate, label, timeout = 9000) => {
    const until = performance.now() + timeout;
    while (performance.now() < until) {
      if (predicate()) return;
      await new Promise(resolve => setTimeout(resolve, 30));
    }
    throw new Error(`Timed out waiting for ${label}`);
  };
  const assert = (condition, message) => { if (!condition) throw new Error(message); };
  const click = selector => {
    const node = document.querySelector(selector);
    assert(node, `Missing control: ${selector}`);
    assert(!node.disabled, `Disabled control: ${selector} (busy=${app.busy})`);
    node.click();
    return node;
  };
  const openDialog = () => document.querySelector('dialog.fi-dialog[open]');
  const waitDialog = () => wait(() => !!openDialog(), 'dialog open');
  const fill = (name, value, dialog = openDialog()) => {
    assert(dialog, 'Expected an open dialog');
    const node = dialog.querySelector(`[name="${CSS.escape(name)}"]`);
    assert(node, `Missing form field: ${name}`);
    node.value = String(value);
    node.dispatchEvent(new Event('input', { bubbles: true }));
    node.dispatchEvent(new Event('change', { bubbles: true }));
    return node;
  };
  const submitDialog = async () => {
    const dialog = openDialog();
    assert(dialog, 'Expected an open dialog to submit');
    dialog.querySelector('form').requestSubmit();
    await wait(() => !dialog.isConnected && !app.busy, 'dialog action to finish');
  };
  const waitIdle = () => wait(() => !app.busy, 'FindIt idle');
  await wait(() => app.status?.enabled && app.catalog && !!document.querySelector('.fi-app'), 'initial app load');
  assert(app.status.can_write, 'fixture needs Desktop write access');

  // Storage locations are first-class records and labels link back to their detail route.
  click('[data-view="locations"]');
  await wait(() => app.view === 'locations', 'locations view');
  click('[data-action="new-location"]');
  await waitDialog();
  fill('name', 'Workbench drawer A3');
  fill('kind', 'drawer');
  fill('description', 'Small parts for the bench');
  await submitDialog();
  await wait(() => app.locations.some(row => row.name === 'Workbench drawer A3'), 'location creation');
  const location = app.locations.find(row => row.name === 'Workbench drawer A3');
  click(`[data-select-location="${location.id}"]`);
  await wait(() => !!document.querySelector('[data-action="print-location-label"]'), 'location label action');
  const locationLink = app.features.workflows.deepLink(location.id, 'location');
  assert(app.features.workflows.findFindItLink(locationLink)?.entity === 'location', 'location deep link must resolve');

  // A physical package family and editable template fields belong to the item record.
  click('[data-view="items"]');
  await waitIdle();
  click('[data-action="new-item"]');
  await waitDialog();
  fill('name', 'LM358 dual op amp');
  fill('category', 'electronic_component');
  fill('template_id', 'op_amp');
  await wait(() => !!openDialog()?.querySelector('[name="property:gain_bandwidth_mhz"]'), 'template fields');
  fill('manufacturer', 'Texas Instruments');
  fill('part_number', 'LM358P');
  fill('unit', 'piece');
  fill('minimum_stock', '2');
  fill('package_family', 'dip');
  fill('pin_count', '8');
  fill('initial_quantity', '12');
  fill('initial_location_id', location.id);
  const bandwidthField = fill('property:gain_bandwidth_mhz', '1.25');
  assert(bandwidthField.closest('label').textContent.includes('MHz') && bandwidthField.validity.valid, 'numeric property needs its unit and fractional input');
  fill('property:supply_voltage_v', '32');
  await submitDialog();
  await wait(() => app.selected?.part_number === 'LM358P' && !app.busy, 'item creation and selection');
  const itemId = app.selected.id;
  let item = await FindItModel.query({ operation: 'get', entity: 'items', id: itemId });
  assert(item.package_family === 'dip' && item.pin_count === 8, 'DIP housing and pin count must persist');
  assert(item.properties?.gain_bandwidth_mhz === 1.25, 'typed template fields must persist as numbers');
  let availability = await FindItModel.query({ operation: 'availability', entity: 'items', id: itemId });
  assert(availability.on_hand === '12' && availability.stocks.length === 1, 'initial stock must be placed at the selected location');

  // Camera/photo capture stores an opaque image id and keeps the atlas as the fallback.
  const canvas = document.createElement('canvas'); canvas.width = 80; canvas.height = 48;
  const brush = canvas.getContext('2d'); brush.fillStyle = '#d5a259'; brush.fillRect(0, 0, 80, 48); brush.fillStyle = '#18342e'; brush.fillRect(8, 8, 64, 32);
  const png = await new Promise(resolve => canvas.toBlob(resolve, 'image/png'));
  assert(png, 'Canvas photo fixture failed');
  const photo = new File([png], 'findit-photo.png', { type: 'image/png' });
  const transfer = new DataTransfer(); transfer.items.add(photo);
  const photoInput = document.querySelector('[data-photo-input]'); photoInput.files = transfer.files;
  photoInput.dispatchEvent(new Event('change', { bubbles: true }));
  await wait(() => !!app.selected?.image_id && !app.busy, 'photo upload');
  item = await FindItModel.query({ operation: 'get', entity: 'items', id: itemId });
  assert(item.image_id && document.querySelector('.fi-detail-art img'), 'photo must override the bundled miniature');
  const savedPhoto = item.image_id;
  await wait(() => !app.busy && !document.querySelector('[data-action="edit-item"]')?.disabled, 'photo refresh before edit');
  click('[data-action="edit-item"]'); await waitDialog(); fill('description', 'Photo and footprint must survive editing'); fill('package_size', '300 mil'); fill('size_system', 'imperial');
  await submitDialog(); await wait(() => !app.busy && app.selected?.description === 'Photo and footprint must survive editing', 'item edit');
  item = await FindItModel.query({ operation: 'get', entity: 'items', id: itemId });
  assert(item.image_id === savedPhoto && item.package_size === '300 mil', 'item edit cleared photo or package-size metadata');

  // Failed stock mutations stay visible and do not alter inventory.
  await wait(() => !app.busy && !document.querySelector('[data-action="stock-issue"]')?.disabled, 'photo refresh completion');
  click('[data-action="stock-issue"]');
  await waitDialog(); fill('stock_id', availability.stocks[0].id); fill('quantity', '999');
  openDialog().querySelector('form').requestSubmit();
  await wait(() => !!openDialog()?.querySelector('[data-error]:not([hidden])')?.textContent, 'stock validation error');
  assert(openDialog().querySelector('[name="quantity"]').value === '999', 'invalid stock entry must retain its draft');
  click('dialog[open] [data-cancel]'); await waitIdle();

  // Ordinary issue and return use the exact stock position and a visible impact preview.
  click('[data-action="stock-issue"]'); await waitDialog();
  fill('stock_id', availability.stocks[0].id); fill('quantity', '1'); await submitDialog();
  await waitDialog(); await submitDialog();
  await wait(() => !app.busy && app.availability?.on_hand === '11', 'ordinary stock issue');
  click(`[data-stock-menu="${availability.stocks[0].id}"]`); await waitDialog();
  click('[data-menu-action="stock-return"]'); await waitDialog();
  fill('stock_id', availability.stocks[0].id); fill('quantity', '1'); await submitDialog();
  await waitDialog(); await submitDialog();
  await wait(() => !app.busy && app.availability?.on_hand === '12', 'ordinary stock return');

  // A released BOM can reserve, release and consume, while completion does not auto-consume.
  click('[data-view="projects"]'); await wait(() => app.view === 'projects' && !app.busy, 'projects view');
  click('[data-action="new-project"]'); await waitDialog(); fill('name', 'Mini synth controller'); fill('description', 'Revision one'); await submitDialog();
  await wait(() => app.project?.name === 'Mini synth controller' && !app.busy, 'project creation');
  click('[data-action="add-project-line"]'); await waitDialog();
  await wait(() => !!openDialog()?.querySelector('[data-pick-item]'), 'part picker results');
  click('[data-pick-item]'); await waitDialog(); fill('quantity', '3'); fill('note', 'DIP package'); await submitDialog();
  await wait(() => app.project?.lines?.length === 1 && !app.busy, 'BOM line creation');
  const bomLine = app.project.lines[0];
  assert(bomLine.item_id === itemId && bomLine.quantity === '3', 'BOM must retain item and exact quantity');
  click('[data-action="release-project"]'); await waitDialog(); click('dialog.fi-dialog[open] [data-submit]');
  await wait(() => app.project?.status === 'released' && !app.busy, 'project release');
  click(`[data-reserve-line="${CSS.escape(bomLine.id)}"]`); await waitDialog(); fill('quantity', '2'); await submitDialog();
  await wait(() => app.project?.reservations?.length === 1 && !app.busy, 'project reservation');
  const reservation = app.project.reservations[0];
  assert(reservation.remaining === '2', 'reservation holds should be explicit in the project detail');
  click(`[data-reservation-action="release"][data-reservation-id="${CSS.escape(reservation.id)}"]`);
  await waitDialog(); fill('quantity', '1'); await submitDialog();
  await wait(() => app.project?.reservations?.[0]?.remaining === '1' && !app.busy, 'partial release');
  click(`[data-reservation-action="consume"][data-reservation-id="${CSS.escape(reservation.id)}"]`);
  await waitDialog(); fill('quantity', '1'); await submitDialog();
  await wait(() => app.project?.reservations?.[0]?.remaining === '0' && !app.busy, 'reserved stock consumption');
  const onHandAfterConsume = (await FindItModel.query({ operation: 'availability', entity: 'items', id: itemId })).on_hand;
  assert(onHandAfterConsume === '11', 'consume must decrement stock exactly once');
  click('[data-action="complete-project"]'); await waitDialog(); click('dialog.fi-dialog[open] [data-submit]');
  await wait(() => app.project?.status === 'completed' && !app.busy, 'project completion');
  assert((await FindItModel.query({ operation: 'availability', entity: 'items', id: itemId })).on_hand === '11', 'completion must not consume stock again');

  // CSV mapping and preview use the real all-or-nothing import endpoint.
  click('[data-view="items"]'); await wait(() => app.view === 'items' && !app.busy, 'workbench return');
  click('[data-action="toggle-filters"]'); click('[data-action="clear-filters"]'); await waitIdle();
  await app.features.workflows.csvWizard('items'); await waitDialog();
  const malformedTransfer = new DataTransfer(); malformedTransfer.items.add(new File([new Uint8Array([110,97,109,101,10,255])], 'invalid.csv', { type: 'text/csv' }));
  let sourceInput = openDialog().querySelector('[data-csv-file]'); sourceInput.files = malformedTransfer.files; sourceInput.dispatchEvent(new Event('change', { bubbles: true }));
  click('dialog[open] [data-csv-action="next"]');
  await wait(() => !!openDialog()?.querySelector('[data-csv-error]:not([hidden])')?.textContent, 'malformed CSV encoding rejected');
  assert(!openDialog().querySelector('[data-csv-map]'), 'invalid UTF-8 reached column mapping');
  click('dialog[open] [data-csv-action="cancel"]');
  await app.features.workflows.csvWizard('items'); await waitDialog();
  const csv = 'name,category,unit,manufacturer,part_number\nNE555 timer,electronic_component,piece,Texas Instruments,NE555P';
  const csvFile = new File([csv], 'parts.csv', { type: 'text/csv' });
  const csvTransfer = new DataTransfer(); csvTransfer.items.add(csvFile);
  const csvInput = openDialog().querySelector('[data-csv-file]'); csvInput.files = csvTransfer.files;
  csvInput.dispatchEvent(new Event('change', { bubbles: true }));
  click('dialog.fi-dialog[open] [data-csv-action="next"]');
  await wait(() => !!openDialog()?.querySelector('[data-csv-map="name"]'), 'CSV column mapping');
  click('dialog.fi-dialog[open] [data-csv-action="preview"]');
  await wait(() => !!openDialog()?.querySelector('.fi-csv-table'), 'CSV preview');
  assert(openDialog().textContent.includes('NE555 timer'), 'CSV preview must show the parsed record');
  click('dialog.fi-dialog[open] [data-csv-action="import"]');
  await wait(() => !openDialog() && app.rows.some(row => row.part_number === 'NE555P'), 'CSV import');
  const exported = await FindItModel.csvExport('items', '', app.signal);
  assert(exported.ok && (await exported.text()).includes('NE555P'), 'CSV export must include imported items');

  // Custom schemas use a visual editor; dates and enums remain typed through capture.
  click('[data-view="templates"]'); await wait(() => app.view === 'templates' && !app.busy, 'template view');
  click('[data-action="new-template"]'); await waitDialog();
  fill('name', 'Calibrated instrument'); fill('category', 'instruments');
  const setProperty = (key, value) => {
    const input = openDialog().querySelector(`[data-field-row]:last-child [data-prop="${key}"]`);
    assert(input, `Missing template control ${key}`); input.value = value;
    input.dispatchEvent(new Event('input', { bubbles: true })); input.dispatchEvent(new Event('change', { bubbles: true }));
  };
  click('[data-add-field]'); setProperty('label', 'Calibration date'); setProperty('key', 'calibration_date'); setProperty('type', 'date');
  click('[data-add-field]'); setProperty('label', 'Range'); setProperty('key', 'measurement_range'); setProperty('type', 'enum'); setProperty('options', 'Low\nHigh');
  await submitDialog();
  await wait(() => app.template?.name === 'Calibrated instrument', 'custom template save');
  const customTemplate = app.template.id;
  click('[data-view="items"]'); await waitIdle();
  click('[data-action="new-item"]'); await waitDialog(); fill('name', 'Bench instrument'); fill('template_id', customTemplate);
  await wait(() => !!openDialog()?.querySelector('[name="property:calibration_date"]'), 'custom template item fields');
  fill('property:calibration_date', '2026-11-01'); fill('property:measurement_range', 'High');
  await submitDialog(); await wait(() => app.selected?.name === 'Bench instrument' && !app.busy, 'custom item capture');
  assert(app.selected.properties.calibration_date === '2026-11-01' && app.selected.properties.measurement_range === 'High', 'custom typed properties must persist');
  const customMatches = await FindItModel.query({ operation: 'search', entity: 'items', template_id: customTemplate, filters: [{ field: 'calibration_date', operator: 'eq', value: '2026-11-01' }] });
  assert(customMatches.total === 1, 'custom date property search must find the instrument');
  const customItemId = app.selected.id;
  click('[data-view="templates"]'); await wait(() => app.view === 'templates' && !app.busy, 'custom template archive view');
  click(`[data-select-template="${customTemplate}"]`); click('[data-action="archive-template"]'); await waitDialog(); await submitDialog();
  await wait(() => !app.busy && !app.catalog.templates.some(template => template.id === customTemplate), 'used custom template archive');
  click('[data-view="items"]'); await waitIdle(); await app.openItem(customItemId);
  assert(document.querySelector('.fi-detail-panel').textContent.includes('Calibration date') && document.querySelector('.fi-detail-panel').textContent.includes('2026-11-01'), 'archiving a used template hid historical fields');
  click('[data-action="new-item"]'); await waitDialog();
  assert(![...openDialog().querySelector('[name="template_id"]').options].some(option => option.value === customTemplate), 'archived template can be assigned to a new item');
  click('dialog[open] [data-cancel]');

  // Deep links are strict same-origin FindIt routes; arbitrary scanned URLs stay passive.
  const itemLink = app.features.workflows.deepLink(itemId, 'item');
  assert(app.features.workflows.findFindItLink(itemLink)?.id === itemId, 'item deep link must resolve');
  assert(!app.features.workflows.findFindItLink('https://attacker.invalid/?app=findit&item=' + itemId), 'external QR links must not resolve');
  assert(!app.features.workflows.findFindItLink(itemLink + '&location=' + location.id), 'ambiguous QR routes must not resolve');
  const finalItem = await FindItModel.query({ operation: 'get', entity: 'items', id: itemId });
  const finalLocation = await FindItModel.query({ operation: 'get', entity: 'locations', id: location.id });
  assert(finalItem.movements?.some(row => row.kind === 'consume'), 'movement history must include the consumed reservation');
  assert(finalLocation.name === location.name, 'location route target remains resolvable');

  // Use the shipped QR composer, then inspect both physical print layouts.
  const labelWork = app.features.workflows.printLabel(finalItem);
  await wait(() => !!document.querySelector('.vd-qr-dialog [data-preview] img'), 'QR label preview', 15000);
  const qrImage = document.querySelector('.vd-qr-dialog [data-preview] img');
  const qrBlob = await (await fetch(qrImage.src)).blob();
  const qrData = await new Promise(resolve => { const reader = new FileReader(); reader.onload = () => resolve(reader.result); reader.readAsDataURL(qrBlob); });
  window.findItQR = { data: qrData.split(',')[1], target: itemLink };
  click('.vd-qr-dialog [data-action="close"]'); await labelWork;
  await wait(() => !!document.querySelector('.fi-print-dialog'), 'physical label layout');
  assert(openDialog().textContent.includes('I-' + itemId.slice(0, 13).toUpperCase()), 'print preview must include the manual identifier');
  const originalOpen = window.open; let printed = false, printWindow;
  window.open = (...args) => { printWindow = originalOpen(...args); if (printWindow) printWindow.print = () => { printed = true; }; return printWindow; };
  try {
    click('.fi-print-dialog input[value="a4"]'); click('.fi-print-dialog [data-print]');
    await wait(() => printed, 'A4 print images ready');
    assert(printWindow.document.querySelectorAll('.label').length === 27, 'A4 must contain 27 labels in the bounded layout');
    assert([...printWindow.document.images].every(image => image.complete && image.naturalWidth > 0), 'print must wait for decoded QR images');
  } finally { window.open = originalOpen; printWindow?.close(); }
  assert(window.errors.length === 0, `browser errors: ${window.errors.join('\n')}`);
  return {
    item: finalItem.id, package: `${finalItem.package_family}${finalItem.pin_count}`, photo: !!finalItem.image_id,
    stock: onHandAfterConsume, project: 'released → reserved → partial release → consume → completed',
    csv: 'preview/import/export', qr: 'item + location deep links validated',
    errors: window.errors.slice()
  };
}
