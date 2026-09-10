import { createUniver, LocaleType, mergeLocales } from '@univerjs/presets';
export { installSheetActions } from './scripts/sheets-actions.js';
import { functionStatistical } from '@univerjs/engine-formula';
export const averageAlias = [functionStatistical.find(entry=>entry[1]==='AVERAGE')[0], 'AVG'];
import { UniverSheetsCorePreset } from '@univerjs/preset-sheets-core';
import { UniverSheetsFilterPreset } from '@univerjs/preset-sheets-filter';
import { UniverSheetsSortPreset } from '@univerjs/preset-sheets-sort';
import { UniverSheetsDataValidationPreset } from '@univerjs/preset-sheets-data-validation';
import { UniverSheetsConditionalFormattingPreset } from '@univerjs/preset-sheets-conditional-formatting';
import { UniverSheetsFindReplacePreset } from '@univerjs/preset-sheets-find-replace';
import { UniverSheetsNotePreset } from '@univerjs/preset-sheets-note';
import { UniverSheetsHyperLinkPreset } from '@univerjs/preset-sheets-hyper-link';
import { UniverSheetsTablePreset } from '@univerjs/preset-sheets-table';

export { createUniver, LocaleType, mergeLocales, UniverSheetsCorePreset, UniverSheetsFilterPreset, UniverSheetsSortPreset, UniverSheetsDataValidationPreset, UniverSheetsConditionalFormattingPreset, UniverSheetsFindReplacePreset, UniverSheetsNotePreset, UniverSheetsHyperLinkPreset, UniverSheetsTablePreset };
