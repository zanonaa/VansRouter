// Compara o NextResponse real com os fixtures já gerados. Dependência local
// fornecida explicitamente: node .../check-next-response.mjs /path/node_modules/next
import { createRequire } from 'node:module';
import { readFile } from 'node:fs/promises';
import { resolve } from 'node:path';
const require = createRequire(resolve(process.argv[2], 'package.json'));
const { NextResponse } = require('./server.js');
for (const name of ['node-http-original.json','pool-http-original.json','pool-read-original.json','provider-create-original.json','provider-update-original.json','bulk-http-original.json','validation-original.json']) {
  const fixtures=JSON.parse(await readFile(new URL('../internal/http/handlers/testdata/'+name,import.meta.url),'utf8'));
  for (const fixture of fixtures) {
    const response=NextResponse.json(fixture.body,{status:fixture.status});
    if(response.status!==fixture.status || response.headers.get('content-type')!=='application/json') throw new Error('NextResponse status/header mismatch');
    if(JSON.stringify(await response.json())!==JSON.stringify(fixture.body)) throw new Error('NextResponse serialization mismatch');
  }
  console.log('PASS NextResponse serialization/status/content-type: '+name);
}
