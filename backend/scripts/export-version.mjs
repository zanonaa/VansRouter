import {readFile,writeFile} from 'node:fs/promises';
const pkg=JSON.parse(await readFile(new URL('../../package.json',import.meta.url),'utf8'));
await writeFile(new URL('../internal/http/handlers/app-version.txt',import.meta.url),pkg.version+'\n');
