// Entry point that exports everything needed by the Code Studio editor.
export {
  EditorView,
  keymap,
  lineNumbers,
  highlightActiveLineGutter,
  highlightSpecialChars,
  drawSelection,
  dropCursor,
  rectangularSelection,
  crosshairCursor,
  highlightActiveLine,
  gutter,
  GutterMarker,
} from '@codemirror/view';
export { EditorState, Prec, Compartment } from '@codemirror/state';
export {
  defaultKeymap,
  history,
  historyKeymap,
  indentWithTab,
  undo,
  redo,
  isolateHistory,
  moveLineUp,
  moveLineDown,
  copyLineUp,
  copyLineDown,
  toggleComment,
  deleteLine,
} from '@codemirror/commands';
export { searchKeymap, highlightSelectionMatches, selectNextOccurrence } from '@codemirror/search';
export {
  javascript,
  javascriptLanguage,
  typescriptLanguage,
  jsxLanguage,
  tsxLanguage,
} from '@codemirror/lang-javascript';
export { python } from '@codemirror/lang-python';
export { go } from '@codemirror/lang-go';
export { cpp } from '@codemirror/lang-cpp';
export { rust } from '@codemirror/lang-rust';
export { json } from '@codemirror/lang-json';
export { html } from '@codemirror/lang-html';
export { css } from '@codemirror/lang-css';
export { markdown } from '@codemirror/lang-markdown';
export { yaml } from '@codemirror/lang-yaml';
export { sql } from '@codemirror/lang-sql';
export { java } from '@codemirror/lang-java';
export { php } from '@codemirror/lang-php';
export { xml } from '@codemirror/lang-xml';
export { shell } from '@codemirror/legacy-modes/mode/shell';
export { dockerFile } from '@codemirror/legacy-modes/mode/dockerfile';
export { toml } from '@codemirror/legacy-modes/mode/toml';
export { oneDark } from '@codemirror/theme-one-dark';
export {
  defaultHighlightStyle,
  syntaxHighlighting,
  indentUnit,
  foldGutter,
  foldKeymap,
  codeFolding,
  foldCode,
  unfoldCode,
  bracketMatching,
  indentOnInput,
  StreamLanguage,
} from '@codemirror/language';
export { autocompletion, closeBrackets, closeBracketsKeymap, completionKeymap } from '@codemirror/autocomplete';
export { lintKeymap, linter, lintGutter, setDiagnostics } from '@codemirror/lint';
