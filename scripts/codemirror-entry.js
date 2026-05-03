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
} from '@codemirror/view';
export { EditorState, Prec, Compartment } from '@codemirror/state';
export { defaultKeymap, history, historyKeymap, indentWithTab } from '@codemirror/commands';
export { searchKeymap, highlightSelectionMatches } from '@codemirror/search';
export {
  javascript,
  javascriptLanguage,
  typescriptLanguage,
  jsxLanguage,
  tsxLanguage,
} from '@codemirror/lang-javascript';
export { python } from '@codemirror/lang-python';
export { go } from '@codemirror/lang-go';
export { rust } from '@codemirror/lang-rust';
export { json } from '@codemirror/lang-json';
export { html } from '@codemirror/lang-html';
export { css } from '@codemirror/lang-css';
export { markdown } from '@codemirror/lang-markdown';
export { oneDark } from '@codemirror/theme-one-dark';
export { defaultHighlightStyle, syntaxHighlighting, indentUnit } from '@codemirror/language';
export { autocompletion, closeBrackets, closeBracketsKeymap, completionKeymap } from '@codemirror/autocomplete';
export { lintKeymap } from '@codemirror/lint';
