// Table registry. DOM-free.
import neon from './neon/index.js';
import leviathan from './leviathan/index.js';
import temple from './temple/index.js';
import observatory from './observatory/index.js';

export const TABLES = Object.freeze({ neon, leviathan, temple, observatory });
export const tableOf = id => TABLES[id] || TABLES.neon;
export default TABLES;
