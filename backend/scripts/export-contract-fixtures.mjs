import { writeFile } from 'node:fs/promises';
const values = [null, true, false, 12, 0, ' pool ', ['a', null, 'b'], {}, { a: 1 }];
const fixtures = values.map(value => ({ value, string: String(value), spread: { ...(value || {}) } }));
await writeFile(new URL('../internal/http/handlers/testdata/js-values.json', import.meta.url), JSON.stringify(fixtures, null, 2) + '\n');
