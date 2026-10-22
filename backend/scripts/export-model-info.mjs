// Run from repository root: node backend/scripts/export-model-info.mjs
import fs from 'node:fs';
import { PROVIDER_MODELS, PROVIDER_MEDIA } from '../../open-sse/providers/index.js';
import REGISTRY from '../../open-sse/providers/registry/index.js';
const endpoints = {llm:'/v1/chat/completions',image:'/v1/images/generations',tts:'/v1/audio/speech',stt:'/v1/audio/transcriptions',embedding:'/v1/embeddings',imageToText:'/v1/chat/completions',webSearch:'/v1/search',webFetch:'/v1/fetch'};
const voices = new Set(['elevenlabs','edge-tts','deepgram','inworld','local-device']);
const result = [];
for (const provider of REGISTRY) {
 const alias = provider.alias || provider.id;
 const media = PROVIDER_MEDIA[provider.id] || {};
 const add = (model,kind) => {
  if (!model.id) return;
  const item = {id:`${alias}/${model.id}`,provider:provider.id,alias,name:model.name || model.id,kind,owned_by:alias,endpoint:endpoints[kind] || null};
  for (const field of ['params','capabilities','options','dimensions','contextWindow']) if (model[field]) item[field]=model[field];
  if (kind==='tts' && voices.has(provider.id)) item.voicesUrl=`/v1/audio/voices?provider=${provider.id}`;
  if (kind==='webSearch' && media.searchConfig) {
   for (const field of ['searchTypes','exaSearchTypes']) if(media.searchConfig[field]) item[field]=media.searchConfig[field];
   if(media.searchConfig.maxMaxResults) item.maxResults=media.searchConfig.maxMaxResults;
   if(media.searchConfig.requiredOptions) item.required=media.searchConfig.requiredOptions;
  }
  if(!result.some(x=>x.id===item.id && x.kind===item.kind)) result.push(item);
 };
 for(const model of PROVIDER_MODELS[alias] || []) add(model,model.kind || model.type || 'llm');
 for(const [config,kind] of [['ttsConfig','tts'],['sttConfig','stt'],['imageConfig','image'],['embeddingConfig','embedding'],['videoConfig','video']]) {
  for(const model of media[config]?.models || []) add(typeof model==='string'?{id:model}:model,kind);
 }
 if(media.searchConfig || media.searchViaChat) add({id:'search',name:`${provider.name || provider.id} Search`,params:['query','max_results','country','language','time_range','domain_filter','search_type']},'webSearch');
 if(media.fetchConfig) add({id:'fetch',name:`${provider.name || provider.id} Fetch`,params:['url','format','max_characters']},'webFetch');
}
fs.writeFileSync(new URL('../internal/providers/model-info.json',import.meta.url),JSON.stringify(result,null,2)+'\n');
