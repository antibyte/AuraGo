// Table registry. DOM-free.
import neon from './neon/index.js';
import leviathan from './leviathan/index.js';
import temple from './temple/index.js';

export const TABLES = Object.freeze({ neon, leviathan, temple });
export const tableOf = id => TABLES[id] || TABLES.neon;
export default TABLES;
