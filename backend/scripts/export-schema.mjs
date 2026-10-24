import { TABLES, SCHEMA_VERSION } from '../../src/lib/db/schema.js';
import { writeFile } from 'node:fs/promises';
await writeFile(new URL('../internal/database/schema.json', import.meta.url), JSON.stringify({ version: SCHEMA_VERSION, tables: TABLES }, null, 2) + '\n');
