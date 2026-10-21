import {readFile,writeFile} from 'node:fs/promises';
const langNames=new Intl.DisplayNames(['en'],{type:'language'});
const countries=new Intl.DisplayNames(['en'],{type:'region'});
const languages={},regions={};
for(let a=97;a<=122;a++)for(let b=97;b<=122;b++){
 const code=String.fromCharCode(a,b);try{languages[code]=langNames.of(code);}catch{}
 const region=code.toUpperCase();try{regions[region]=countries.of(region);}catch{}
}
for(let n=0;n<1000;n++){const region=String(n).padStart(3,'0');try{regions[region]=countries.of(region);}catch{}}
const source=await readFile(new URL('../../open-sse/handlers/ttsProviders/gemini.js',import.meta.url),'utf8');
const match=source.match(/const PREBUILT_VOICES = (\[[\s\S]*?\]);/);
if(!match)throw new Error('Gemini voices not found');
const list=new Function('return '+match[1])();
const gemini=list.map(v=>({voice_id:v.id,name:v.id,labels:{language:v.lang,gender:v.gender}}));
await writeFile(new URL('../internal/http/handlers/voice-metadata.json',import.meta.url),JSON.stringify({languages,regions,gemini},null,2)+'\n');
